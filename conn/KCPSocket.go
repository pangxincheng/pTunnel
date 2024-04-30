package conn

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/xtaci/kcp-go/v5"
)

type KCPSocket struct {
	socket    *kcp.UDPSession
	reader    *bufio.Reader
	closeFlag bool
}

func (socket *KCPSocket) GetSocket() *kcp.UDPSession {
	return socket.socket
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

func (socket *KCPSocket) RemoteAddr() string {
	return socket.socket.RemoteAddr().String()
}

func (socket *KCPSocket) LocalAddr() string {
	return socket.socket.LocalAddr().String()
}

func (socket *KCPSocket) Address() (string, string) {
	return socket.socket.LocalAddr().String(), socket.socket.RemoteAddr().String()
}

type KCPListener struct {
	listener *kcp.Listener
}

func (listener *KCPListener) Listener() *kcp.Listener {
	return listener.listener
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

func NewKCPSocket(addr string, port int, network string) (Socket, error) {
	socket := &KCPSocket{}
	serverAddr, err := net.ResolveUDPAddr(network, fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	kcpConn, err := kcp.DialWithOptions(serverAddr.String(), nil, 10, 3)
	if err != nil {
		return nil, err
	}
	socket.socket = kcpConn
	socket.reader = bufio.NewReader(socket.socket)
	socket.closeFlag = false
	return socket, nil
}

func NewKCPSocketV2(laddr *net.UDPAddr, raddr *net.UDPAddr) (Socket, error) {
	socket := &KCPSocket{}
	network := "udp4"
	if raddr.IP.To4() == nil {
		network = "udp"
	}

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
	socket.socket = kcpConn
	socket.reader = bufio.NewReader(socket.socket)
	socket.closeFlag = false
	return socket, nil
}

func NewKCPListener(addr string, port int, network string) (Listener, error) {
	listener := &KCPListener{}
	serverAddr, err := net.ResolveUDPAddr(network, fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	kcpListener, err := kcp.ListenWithOptions(serverAddr.String(), nil, 10, 3)
	if err != nil {
		return nil, err
	}
	listener.listener = kcpListener
	return listener, nil
}

func NewKCPListenerV2(serverAddr *net.UDPAddr) (Listener, error) {
	listener := &KCPListener{}
	kcpListener, err := kcp.ListenWithOptions(serverAddr.String(), nil, 10, 3)
	if err != nil {
		return nil, err
	}
	listener.listener = kcpListener
	return listener, nil
}
