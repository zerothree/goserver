package goserver

import (
    "bufio"
    "errors"
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
        s.close()
        close(s.outgoing)

        if e := recover(); e != nil {
        }
    }()

    s.server.callback.OnConnected(s, s.conn.RemoteAddr().String())
    defer s.server.callback.OnClosed(s)

    bufconn := bufio.NewReader(s.conn)
    headerBuf := make([]byte, s.server.requestHeaderLength, s.server.requestHeaderLength)

    buffLen := 1024
    buff := make([]byte, buffLen)

    var err error
    var bodyLength int
    for {
        if _, err = io.ReadFull(bufconn, buff[:s.server.requestHeaderLength]); err != nil {
            break
        }

        if bodyLength, err = s.server.callback.OnRequestHeaderDataRecved(s, buff[:s.server.requestHeaderLength]); err != nil {
            break
        }

        bodyBuff = buff
        if bodyLength > buffLen {
            bodyBuff = make([]byte, bodyLength)
        }
        if _, err = io.ReadFull(bufconn, bodyBuff[:bodyLength]); err != nil {
            break
        }

        if err = s.server.callback.OnRequestBodyDataRecved(s, bodyBuff[:bodyLength]); err != nil {
            break
        }
    }
}

func (s *session) send() {
    for _, p := range s.outgoing {
        _, err := s.conn.Write(p)
        if err != nil {
            s.close()
            break
        }
    }
}

// close connection
func (s *session) close() error {
    return s.conn.close()
}

// write data to outgoing channel
// write data to a closed outgoing will return error. write data to a buffer-fulled outgoing will return error too.
func (s *session) Write(p []byte) (n int, err error) {
    defer func() {
        err = recover()
    }()

    buff := make([]byte, len(p))
    copy(buff, p)

    select {
    case s.outgoing <- buff:
        return len(p), nil
    default:
        return 0, errors.New("write channel is full")
    }
}
