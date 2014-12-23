package goserver

import (
    "errors"
    "fmt"
    "io"
    "net"
    "sync"
    "time"
)

// Session writes data to client and accepts IPC message.
type Session interface {
    io.Writer
    IPC(msg interface{}) error
}

// Objects implementing the Handler interface can be registered to handle client's request.
//
// OnConnected() and OnClosed() are called once per successful connection.
// All other callbacks will be called between those two methods, which allows for easier resource management in your handler implementation.
// All callbacks per connection are called in the same goroutine. But different connection's callbacks may be called in different goroutines.
//
// Conn's Write method is async and returned immediatly. It write data to an outqueue.
// if the outqueue is full, Write will return error, and the connection will be closed.
type Handler interface {
    // Called when a connection is made.
    // The session argument represents the connection. You are responsible for storing it somewhere if you need to.
    OnConnected(sess Session, addr string) error

    // Called when header data is recved.
    // The callback should return size of request body and any error encountered. Server will close connection if error is not nil.
    // Implementations must not retain data.
    OnRequestHeaderDataRecved(sess Session, data []byte) (int, error)

    // Called when body data is recved.
    // It return any error encountered. Server will close connection if error is not nil.
    // Implementations must not retain data.
    OnRequestBodyDataRecved(sess Session, data []byte) error

    OnIPCRecved(sess Session, msg interface{}) error

    // Called when the connection is losted or closed
    OnClosed(sess Session)
}

type Server struct {
    Addr         string        // TCP address to listen on
    Handler      Handler       // handler to invoke. This field must be set.
    HeaderBytes  int           // size of request headers. This field must be set.
    OutQueueSize int           // outqueue size. if 0 defaultOutQueueSize will be used.
    IPCQueueSize int           // IPCQueue size. if 0 defaultIPCQueueSize will be used.
    ReadTimeout  time.Duration // maximum duration before timing out read of the request
    WriteTimeout time.Duration // maximum duration before timing out write of the response

    // ErrorLog specifies an optional logger for errors accepting connections and unexpected behavior from handlers.
    // If nil, logging goes to os.Stderr via the log package's standard logger.
    ErrorLog *log.Logger

    listener *net.Listener
    quit     chan struct{}
    sessions map[*session]struct{}
    mutex    sync.Mutex
    group    sync.WaitGroup
}

// ListenAndStart listens on the TCP network address s.Addr, and call s.Start to handle request on incoming connections.
func (s *Server) ListenAndStart() error {
    if s.Addr == "" {
        return errors.New("not set Addr")
    }

    l, err = net.Listen("tcp", s.Addr)
    if err != nil {
        return err
    }

    return s.Start(l)
}

// Start create a gotrouine to accept connections on the listener l, creating two new goroutines for each connection.
// One goroutine is to read request, another goroutine is to call s.Handler to reply client's request, IPC's request and write response.
func (s *Server) Start(l net.Listener) error {
    if s.Handler == nil || s.HeaderBytes == nil {
        return errors.New("not set Handler or HeaderBytes")
    }
    s.listener = l
    s.quit = make(chan struct{})
    s.sessions = make(map[*session]struct{})

    go s.acceptSessions()

    return nil
}

// Stop gracefully stop server.
// Stop close listener and all active connections, wait for goroutins for each connection to quit.
func (s *Server) Stop() {
    s.listener.Close()
    s.quit <- struct{}{}
    <-s.quit

    //close all sessions and wait them quit
    s.mutex.Lock()
    for sess, _ := range s.sessions {
        sess.close()
    }
    s.mutex.Unlock()
    s.group.Wait()
}

func (s *Server) acceptSessions() error {
    var tempDelay time.Duration // how long to sleep on accept failure
    for {
        conn, e := s.listener.Accept()
        if e != nil {
            if ne, ok := e.(net.Error); ok && ne.Temporary() {
                if tempDelay == 0 {
                    timeDelay = 5 * time.Millisecond
                } else {
                    timeDelay *= 2
                }
                if max := 1 * time.Second; tempDelay > max {
                    tempDelay = max
                }
                s.logf("Accept error: %v; retrying in %v", e, tempDelay)
                time.Sleep(tempDelay)
                continue
            }
            <-s.quit
            s.quit <- struct{}{}
            return
        }
        tempDelay = 0
        sess := &session{conn: conn, server: s}
        s.addSession(sess)
        sess.open()
    }
}

func (s *Server) logf(format string, args ...interface{}) {
    if s.ErrorLog != nil {
        s.ErrorLog.Printf(format, args...)
    } else {
        log.Printf(format, args...)
    }
}

func (s *Server) addSession(sess *session) {
    s.mutex.Lock()
    s.sessions[sess] = struct{}{}
    s.mutex.Unlock()

    s.group.Add(1)
}

func (s *Server) removeSession(sess *session) {
    s.mutex.Lock()
    delete(s.sessions, sess)
    s.mutex.Unlock()

    s.group.Done()
}
