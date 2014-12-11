package goserver

import (
    "sync"
)

type buffer []byte

const defaultBufferLen = 1024

var bufferFree = sync.Pool{
    New: func() interface{} { return make([]byte, defaultBufferLen, defaultBufferLen) },
}
