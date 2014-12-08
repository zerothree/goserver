package goserver

import (
    "net"
)

type Session struct {
    conn     *net.TCPConn
    incoming chan []byte
    outgoing chan []byte
}

func (s *Session) Start() {
    go s.recv()
}

func (s *Session) recv() {
    for {

    }
}
