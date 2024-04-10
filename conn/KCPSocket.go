package conn

import (
	"bufio"
	"fmt"
	"github.com/xtaci/kcp-go/v5"
	"net"
)

type KCPSocket struct {
	socket    *kcp.UDPSession
	reader    *bufio.Reader
	closeFlag bool
}

func (socket *KCPSocket) Close() error {
	socket.closeFlag = true
	return socket.socket.Close()
}

func (socket *KCPSocket) Write(p []byte) (n int, err error) {
	return socket.socket.Write(p)
}

func (socket *KCPSocket) Read(p []byte) (n int, err error) {
	return socket.socket.Read(p)
}

func (socket *KCPSocket) ReadLine() (data []byte, err error) {
	data, err = socket.reader.ReadBytes('\n')
	return
}

func (socket *KCPSocket) WriteLine(data []byte) (err error) {
	_, err = socket.socket.Write(append(data, '\n'))
	return
}

type KCPListener struct {
	listener *kcp.Listener
}

func (listener *KCPListener) AcceptKCP() (*KCPSocket, error) {
	kcpConn, err := listener.listener.AcceptKCP()
	if err != nil {
		return nil, err
	}
	socket := &KCPSocket{}
	socket.socket = kcpConn
	socket.reader = bufio.NewReader(socket.socket)
	socket.closeFlag = false
	return socket, nil
}

func (listener *KCPListener) Close() error {
	return listener.listener.Close()
}

func (listener *KCPListener) Accept() (Socket, error) {
	return listener.AcceptKCP()
}

func (listener *KCPListener) Network() string {
	return "kcp"
}

func (listener *KCPListener) Address() (string, int) {
	return listener.listener.Addr().(*net.UDPAddr).IP.String(), listener.listener.Addr().(*net.UDPAddr).Port
}

func NewKCPSocket(addr string, port int) (Socket, error) {
	socket := &KCPSocket{}
	kcpConn, err := kcp.DialWithOptions(fmt.Sprintf("%s:%d", addr, port), nil, 10, 3)
	if err != nil {
		return nil, err
	}
	socket.socket = kcpConn
	socket.reader = bufio.NewReader(socket.socket)
	socket.closeFlag = false
	return socket, nil
}

func NewKCPListener(addr string, port int) (Listener, error) {
	listener := &KCPListener{}
	serverAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", addr, port))
	kcpListener, err := kcp.ListenWithOptions(serverAddr.String(), nil, 10, 3)
	if err != nil {
		return nil, err
	}
	listener.listener = kcpListener
	return listener, nil
}
