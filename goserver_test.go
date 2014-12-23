package goserver

import (
    "testing"
)

type dummyAddr string
type oneConnListener struct {
    conn net.Conn
}

func (l *oneConnListener) Accept() (c net.Conn, err error) {
    c = l.conn
    if c == nil {
        err = io.EOF
        return
    }
    err = nil
    l.conn = nil
    return
}

func (l *oneConnListener) Close() error {
    return nil
}

func (l *oneConnListener) Addr() net.Addr {
    return dummyAddr("test-address")
}

func (a dummyAddr) Network() string {
    return string(a)
}

func (a dummyAddr) String() string {
    return string(a)
}

type noopConn struct{}

func (noopConn) LocalAddr() net.Addr                { return dummyAddr("local-addr") }
func (noopConn) RemoteAddr() net.Addr               { return dummyAddr("remote-addr") }
func (noopConn) SetDeadline(t time.Time) error      { return nil }
func (noopConn) SetReadDeadline(t time.Time) error  { return nil }
func (noopConn) SetWriteDeadline(t time.Time) error { return nil }

type rwTestConn struct {
    io.Reader
    io.Writer
    noopConn

    closeFunc func() error // called if non-nil
    closec    chan bool    // else, if non-nil, send value to it on close
}

func (c *rwTestConn) Close() error {
    if c.closeFunc != nil {
        return c.closeFunc()
    }
    select {
    case c.closec <- true:
    default:
    }
    return nil
}

type testConn struct {
    readBuf  bytes.Buffer
    writeBuf bytes.Buffer
    closec   chan bool // if non-nil, send value to it on close
    noopConn
}

func (c *testConn) Read(b []byte) (int, error) {
    return c.readBuf.Read(b)
}

func (c *testConn) Write(b []byte) (int, error) {
    return c.writeBuf.Write(b)
}

func (c *testConn) Close() error {
    select {
    case c.closec <- true:
    default:
    }
    return nil
}

func TestGoServer_StartFailByResolveInvalidPort(t *testing.T) {
    s := NewServer(0, nil)
    err := s.Start(9999999)
    if err == nil {
        t.Fatalf("err should not be nil")
    }
}

func TestGoServer_StartFailByListenError(t *testing.T) {
    port := 65289

    s := NewServer(0, nil)
    s.Start(port)
    s1 := NewServer(0, nil)
    err := s1.Start(port)
    if err == nil {
        t.Fatalf("err should not be nil")
    }
}
