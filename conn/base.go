package conn

import (
	"io"
)

type Socket interface {
	io.ReadWriteCloser
	ReadLine() ([]byte, error)
	WriteLine([]byte) error
}

type Listener interface {
	io.Closer
	Accept() (Socket, error)
	Network() string
	Address() (string, int)
}
