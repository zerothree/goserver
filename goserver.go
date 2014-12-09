package goserver

import (
    "fmt"
    "io"
    "net"
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
    map[net.TCPConn]
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

        session := &Session{conn}
        session.Start()
    }
}
