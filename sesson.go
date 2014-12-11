package goserver

import (
    "io"
    "net"
)

type session struct {
    conn     *net.TCPConn
    server   *Server
    outgoing chan []byte
}

func (s *session) open() {
    s.outgoing = make(chan []byte, 100)
    go s.recv()
    go s.send()
}

func (s *session) recv() {
    defer func() {
        s.server.removeSession(s)
        s.Close()
        if e := recover(); e != nil {
        }
    }()

    s.server.callback.OnConnected(s, s.conn.RemoteAddr().String())
    defer s.server.callback.OnClosed(s)

    b := bufferFree.Get()
    defer bufferFree.Put(b)
    for {
        n, err := s.conn.Read(b)
        if n == 0 || err != nil {
            break
        }

        s.server.callback.OnDataRecved(s, b[:n])
    }
}

func (s *session) send() {
    for _, p := range s.outgoing {
        _, err := s.conn.Write(p)
        if err != nil {
            s.Close()
            break
        }
    }
}

// close connection
func (s *session) Close() error {
    return s.conn.Close()
}

// write data to connection
func (s *session) Write(p []byte) (n int, err error) {
    select {
    case outgoing <- p:
        return len(p), nil
    default:
        return 0, io.ErrShortWrite
    }
}
