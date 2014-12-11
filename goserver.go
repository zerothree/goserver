package goserver

import (
    "fmt"
    "io"
    "net"
    "sync"
)

type Callback interface {
    OnConnected(conn io.WriteCloser, addr string)

    // It returns the length of request body and any error encountered. server will close connection if error is not nil.
    OnRequestHeaderDataRecved(conn io.WriteCloser, data []byte) (int, error)

    //
    // It return any error encountered. if must return a non-nil error if parsing data to request body is failed.
    // server will close connection if error is not nil.
    //
    // Implementations must not retain data.
    OnRequestBodyDataRecved(conn io.WriteCloser, data []byte) error
    OnClosed(conn io.WriteCloser)
}

type Server struct {
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

    s.quit = make(chan []struct{})
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
        sess.Close()
    }
    s.mutex.Unlock()
    s.group.Wait()
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
