package conn

import (
	"bufio"
	"fmt"
	"net"
)

type TCPSocket struct {
	socket    *net.TCPConn
	reader    *bufio.Reader
	closeFlag bool
}

func (socket *TCPSocket) Close() error {
	socket.closeFlag = true
	return socket.socket.Close()
}

func (socket *TCPSocket) Write(p []byte) (n int, err error) {
	return socket.socket.Write(p)
}

func (socket *TCPSocket) Read(p []byte) (n int, err error) {
	return socket.socket.Read(p)
}

func (socket *TCPSocket) ReadLine() (data []byte, err error) {
	data, err = socket.reader.ReadBytes('\n')
	return
}

func (socket *TCPSocket) WriteLine(data []byte) (err error) {
	_, err = socket.socket.Write(append(data, '\n'))
	return
}

type TCPListener struct {
	listener *net.TCPListener
}

func (listener *TCPListener) AcceptTCP() (*TCPSocket, error) {
	conn, err := listener.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return &TCPSocket{
		socket:    conn,
		reader:    bufio.NewReader(conn),
		closeFlag: false,
	}, nil
}

func (listener *TCPListener) Close() error {
	return listener.listener.Close()
}

func (listener *TCPListener) Accept() (Socket, error) {
	return listener.AcceptTCP()
}

func (listener *TCPListener) Network() string {
	return "tcp"
}

func (listener *TCPListener) Address() (string, int) {
	return listener.listener.Addr().(*net.TCPAddr).IP.String(), listener.listener.Addr().(*net.TCPAddr).Port
}

func NewTCPSocket(addr string, port int, network string) (Socket, error) {
	socket := &TCPSocket{}
	serverAddr, err := net.ResolveTCPAddr(network, fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	socket.socket, err = net.DialTCP(network, nil, serverAddr)
	if err != nil {
		return nil, err
	}
	socket.reader = bufio.NewReader(socket.socket)
	socket.closeFlag = false
	return socket, nil
}

func NewTCPListener(addr string, port int, network string) (Listener, error) {
	listener := &TCPListener{}
	serverAddr, err := net.ResolveTCPAddr(network, fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	listener.listener, err = net.ListenTCP(network, serverAddr)
	if err != nil {
		return nil, err
	}
	return listener, nil
}
