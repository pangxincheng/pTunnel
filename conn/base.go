package conn

import (
	"io"
	"net"
)

type Socket interface {
	io.ReadWriteCloser
	ReadLine() ([]byte, error)
	WriteLine([]byte) error
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Address() (net.Addr, net.Addr)
}

type Listener interface {
	io.Closer
	Accept() (Socket, error)
	Network() string
	Address() net.Addr
}
