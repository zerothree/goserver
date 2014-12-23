package goserver

import (
    "bufio"
    "errors"
    "io"
    "net"
    "runtime"
)

type session struct {
    conn         net.Conn
    server       *Server
    outgoing     chan []byte
    ipcs         chan interface{}
    incomingHead chan []byte
    incomingBody chan []byte
    headLenChan  chan int
}

var (
    defaultOutQueueSize = 100
    defaultIPCQueueSize = 100
)

func (s *session) open() {
    outgoingSize := defaultOutQueueSize
    if s.server.OutQueueSize != 0 {
        outgoingSize = s.server.OutQueueSize
    }
    ipcsSize := defaultIPCQueueSize
    if s.server.IPCQueueSize != 0 {
        ipcsSize = s.server.IPCQueueSize
    }

    s.outgoing = make(chan []byte, outgoingSize)
    s.ipcs = make(chan interface{}, ipcsSize)
    s.incomingHead = make(chan []byte, 1)
    s.incomingBody = make(chan []byte, 1)
    s.headLenChan = make(chan int, 1)
    go s.recv()
    go s.mux()
}

func (s *session) recv() {
    defer func() {
        close(s.incomingHead)
        close(s.incomingBody)
    }()

    bufconn := bufio.NewReader(s.conn)

    buffLen := 1024
    buff := make([]byte, buffLen)

    var err error
    var bodyLength int
    for {
        if s.server.ReadTimeout != 0 {
            s.conn.SetReadDeadline(time.Now().Add(s.server.ReadTimeout))
        }
        if _, err = io.ReadFull(bufconn, buff[:s.server.HeaderBytes]); err != nil {
            break
        }
        s.incomingHead <- buff[:s.server.HeaderBytes]
        bodyLength, ok := <-s.headLenChan
        if !ok {
            break
        }

        bodyBuff := buff
        if bodyLength > buffLen {
            bodyBuff = make([]byte, bodyLength)
        }
        if _, err = io.ReadFull(bufconn, bodyBuff[:bodyLength]); err != nil {
            break
        }
        s.incomingBody <- bodyBuff[:bodyLength]
    }
}

func (s *session) send(dadta []byte) error {
    if s.server.WriteTimeout != 0 {
        s.conn.SetWriteDeadline(time.Now().Add(s.server.WriteTimeout))
    }
    _, err := s.conn.Write(data)
    return err
}

func (s *session) mux() {
    defer func() {
        s.server.removeSession(s)
        s.close()
        close(s.headLenChan)

        if e := recover(); e != nil {
            const size = 64 << 10
            buf := make([]byte, size)
            buf = buf[:runtime.Stack(buf, false)]
            c.server.logf("mux: panic %v: %v\n%s", s.conn.remoteAddr, err, buf)
        }
    }()

    err := s.server.Handler.OnConnected(s, s.conn.RemoteAddr())
    defer s.server.Handler.OnClosed(s)

    if err != nil {
        return
    }

    for {
        select {
        case headData, ok := <-s.incomingHead:
            if !ok {
                break
            }
            bodyLength, err := s.server.Handler.OnRequestHeaderDataRecved(s, headData)
            if err != nil {
                break
            }
            s.headLenChan <- bodyLength
        case bodyData, ok := <-s.incomingBody:
            if !ok {
                break
            }
            err := s.server.Handler.OnRequestBodyDataRecved(s, bodyData)
            if err != nil {
                break
            }
        case ipcMsg := <-s.ipcs:
            err := s.server.Handler.OnIPCRecved(s, ipcMsg)
            if err != nil {
                break
            }
        case data := <-s.outgoing:
            err := s.send(data)
            if err != nil {
                break
            }
        default:

        }
        if !ok {
            break
        }
    }
}

// close connection
func (s *session) close() error {
    return s.conn.Close()
}

// write data to outgoing channel
// write data to a closed outgoing will return error. write data to a buffer-fulled outgoing will return error too.
func (s *session) Write(p []byte) (n int, err error) {
    buff := make([]byte, len(p))
    copy(buff, p)

    select {
    case s.outgoing <- buff:
        return len(p), nil
    default:
        // slow client
        s.close()
        return 0, errors.New("write channel is full")
    }
}

func (s *session) IPC(msg interface{}) error {
    select {
    case s.ipcs <- msg:
        return nil
    default:
        // slow client
        s.close()
        return errors.New("ipc channel is full")
    }
}
