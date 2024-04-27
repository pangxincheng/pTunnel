package conn

import (
	"io"
)

type Socket interface {
	io.ReadWriteCloser
	ReadLine() ([]byte, error)
	WriteLine([]byte) error
	RemoteAddr() string
	LocalAddr() string
	Address() (string, string)
}

type Listener interface {
	io.Closer
	Accept() (Socket, error)
	Network() string
	Address() (string, int)
}
