package goserver

import (
    "fmt"
    "io"
    "net"
    "sync"
)

type Callback interface {
    OnConnected(conn io.WriteCloser, addr string)
    OnDataRecved(conn io.WriteCloser, data []byte)
    OnClosed(conn io.WriteCloser)
}

type Server struct {
    listener *net.TCPListener
    quit     chan struct{}
    callback Callback
    sessions map[*session]struct{}
    mutex    sync.Mutex
    group    sync.WaitGroup
}

func (s *Server) RegCallback(cb Callback) {
    s.callback = cb
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

func (s *Server) Stop() {
    s.listener.Close()
    s.quit <- struct{}{}
    <-s.quit

    s.mutex.Lock()
    for sess, _ := range s.sessions {
        sess.Close()
    }
    s.mutex.Unlock()
    //wwait all conns routine quit
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

        sess := &session{conn}
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
