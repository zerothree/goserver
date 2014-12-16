package goserver

import (
    "fmt"
    "io"
    "net"
    "sync"
    "time"
)

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
    OnConnected(session io.Writer, addr string)

    // Called when header data is recved.
    // The callback should return size of request body and any error encountered. Server will close connection if error is not nil.
    // Implementations must not retain data.
    OnRequestHeaderDataRecved(session io.Writer, data []byte) (int, error)

    // Called when body data is recved.
    // It return any error encountered. Server will close connection if error is not nil.
    // Implementations must not retain data.
    OnRequestBodyDataRecved(session io.Writer, data []byte) error

    // Called when the connection is losted or closed.
    OnClosed(session io.Writer, err error)
}

type Server struct {
    Addr         string        // TCP address to listen on
    Handler      Handler       // handler to invoke
    ReadTimeout  time.Duration // maximum duration before timing out read of the request
    WriteTimeout time.Duration // maximum duration before timing out write of the response
    OutQueueSize int           // outqueue size. if 0 defaultOutQueueSize will be used.
    HeaderBytes  int           // size of request headers

    // ErrorLog specifies an optional logger for errors accepting connections and unexpected behavior from handlers.
    // If nil, logging goes to os.Stderr via the log package's standard logger.
    ErrorLog *log.Logger

    listener            *net.TCPListener
    quit                chan struct{}
    callback            Callback
    sessions            map[*session]struct{}
    mutex               sync.Mutex
    group               sync.WaitGroup
    requestHeaderLength int
}

func NewServer(requestHeaderLength int, cb Callback) *Server {
    s := &Server{}
    s.requestHeaderLength = requestHeaderLength
    s.callback = cb

    s.quit = make(chan struct{})
    s.sessions = make(map[*session]struct{})

    return s
}

func (s *Server) Start(port int) error {
    tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf(":%d", port))
    if err != nil {
        return err
    }

    s.listener, err = net.ListenTCP("tcp", tcpAddr)
    if err == nil {
        go s.acceptSessions()
    }

    return err
}

// gracefully stop server
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

func (s *Server) Serve(l net.Listener) error {
    defer l.Close()
    var tempDelay time.Duration // how long to sleep on accept failure
    for {
        rw, e := l.Accept()
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
            return e
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

func (s *Server) acceptSessions() {
    for {
        conn, err := s.listener.AcceptTCP()
        if err != nil {
            select {
            case <-s.quit:
                s.quit <- struct{}{}
                return
            default:
            }
            continue
        }

        sess := &session{conn: conn, server: s}
        s.addSession(sess)
        sess.open()
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
