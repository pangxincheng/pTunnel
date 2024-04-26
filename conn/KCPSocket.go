package conn

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"net"

	"github.com/pkg/errors"
	"github.com/xtaci/kcp-go/v5"
)

type KCPSocket struct {
	Socket    *kcp.UDPSession
	reader    *bufio.Reader
	closeFlag bool
}

func (socket *KCPSocket) Close() error {
	socket.closeFlag = true
	return socket.Socket.Close()
}

func (socket *KCPSocket) Write(p []byte) (n int, err error) {
	return socket.Socket.Write(p)
}

func (socket *KCPSocket) Read(p []byte) (n int, err error) {
	return socket.Socket.Read(p)
}

func (socket *KCPSocket) ReadLine() (data []byte, err error) {
	data, err = socket.reader.ReadBytes('\n')
	return
}

func (socket *KCPSocket) WriteLine(data []byte) (err error) {
	_, err = socket.Socket.Write(append(data, '\n'))
	return
}

func (socket *KCPSocket) RemoteAddr() net.Addr {
	return socket.Socket.RemoteAddr()
}

func (socket *KCPSocket) LocalAddr() net.Addr {
	return socket.Socket.LocalAddr()
}

func (socket *KCPSocket) Address() (net.Addr, net.Addr) {
	return socket.Socket.LocalAddr(), socket.Socket.RemoteAddr()
}

type KCPListener struct {
	Listener *kcp.Listener
}

func (listener *KCPListener) AcceptKCP() (*KCPSocket, error) {
	kcpConn, err := listener.Listener.AcceptKCP()
	if err != nil {
		return nil, err
	}
	socket := &KCPSocket{}
	socket.Socket = kcpConn
	socket.reader = bufio.NewReader(socket.Socket)
	socket.closeFlag = false
	return socket, nil
}

func (listener *KCPListener) Close() error {
	return listener.Listener.Close()
}

func (listener *KCPListener) Accept() (Socket, error) {
	return listener.AcceptKCP()
}

func (listener *KCPListener) Network() string {
	return "kcp"
}

func (listener *KCPListener) Address() net.Addr {
	return listener.Listener.Addr()
}

func NewKCPSocket(laddr *net.UDPAddr, raddr *net.UDPAddr, network string) (Socket, error) {
	socket := &KCPSocket{}
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var convid uint32
	binary.Read(rand.Reader, binary.LittleEndian, &convid)
	kcpConn, err := kcp.NewConn3(convid, raddr, nil, 10, 3, conn)
	if err != nil {
		return nil, err
	}
	socket.Socket = kcpConn
	socket.reader = bufio.NewReader(socket.Socket)
	socket.closeFlag = false
	return socket, nil
}

func NewKCPListener(addr *net.UDPAddr) (Listener, error) {
	listener := &KCPListener{}
	kcpListener, err := kcp.ListenWithOptions(addr.String(), nil, 10, 3)
	if err != nil {
		return nil, err
	}
	listener.Listener = kcpListener
	return listener, nil
}
