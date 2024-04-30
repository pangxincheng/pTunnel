package conn

import (
	"bufio"
	"net"
)

type TCPSocket struct {
	Socket    *net.TCPConn
	reader    *bufio.Reader
	closeFlag bool
}

func (socket *TCPSocket) Close() error {
	socket.closeFlag = true
	return socket.Socket.Close()
}

func (socket *TCPSocket) Write(p []byte) (n int, err error) {
	return socket.Socket.Write(p)
}

func (socket *TCPSocket) Read(p []byte) (n int, err error) {
	return socket.Socket.Read(p)
}

func (socket *TCPSocket) ReadLine() (data []byte, err error) {
	data, err = socket.reader.ReadBytes('\n')
	return
}

func (socket *TCPSocket) WriteLine(data []byte) (err error) {
	_, err = socket.Socket.Write(append(data, '\n'))
	return
}

func (socket *TCPSocket) RemoteAddr() net.Addr {
	return socket.Socket.RemoteAddr()
}

func (socket *TCPSocket) LocalAddr() net.Addr {
	return socket.Socket.LocalAddr()
}

func (socket *TCPSocket) Address() (net.Addr, net.Addr) {
	return socket.Socket.LocalAddr(), socket.Socket.RemoteAddr()
}

type TCPListener struct {
	Listener *net.TCPListener
}

func (listener *TCPListener) AcceptTCP() (*TCPSocket, error) {
	conn, err := listener.Listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return &TCPSocket{
		Socket:    conn,
		reader:    bufio.NewReader(conn),
		closeFlag: false,
	}, nil
}

func (listener *TCPListener) Close() error {
	return listener.Listener.Close()
}

func (listener *TCPListener) Accept() (Socket, error) {
	return listener.AcceptTCP()
}

func (listener *TCPListener) Network() string {
	return "tcp"
}

func (listener *TCPListener) Address() net.Addr {
	return listener.Listener.Addr()
}

func NewTCPSocket(laddr *net.TCPAddr, raddr *net.TCPAddr, network string) (Socket, error) {
	conn, err := net.DialTCP(network, laddr, raddr)
	if err != nil {
		return nil, err
	}
	return &TCPSocket{
		Socket:    conn,
		reader:    bufio.NewReader(conn),
		closeFlag: false,
	}, nil
}

func NewTCPListener(addr *net.TCPAddr, network string) (Listener, error) {
	listener, err := net.ListenTCP(network, addr)
	if err != nil {
		return nil, err
	}
	return &TCPListener{
		Listener: listener,
	}, nil
}
