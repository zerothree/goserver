package goserver

import (
    "testing"
)

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
