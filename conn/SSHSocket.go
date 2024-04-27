package conn

import (
	"errors"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

type SSHSocket struct {
	sshClient *ssh.Client
	socket    *net.Conn
}

func (socket *SSHSocket) Read(p []byte) (n int, err error) {
	return (*socket.socket).Read(p)
}

func (socket *SSHSocket) Write(p []byte) (n int, err error) {
	return (*socket.socket).Write(p)
}

func (socket *SSHSocket) Close() error {
	err := (*socket.socket).Close()
	if err != nil {
		return err
	}
	return socket.sshClient.Close()
}

// ReadLine : as SSHSocket is only used for tunnel, it's not necessary to implement ReadLine
func (socket *SSHSocket) ReadLine() ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (socket *SSHSocket) WriteLine(bytes []byte) error {
	_, err := (*socket.socket).Write(append(bytes, '\n'))
	return err
}

func (socket *SSHSocket) RemoteAddr() string {
	return (*socket.socket).RemoteAddr().String()
}

func (socket *SSHSocket) LocalAddr() string {
	return (*socket.socket).LocalAddr().String()
}

func (socket *SSHSocket) Address() (string, string) {
	return (*socket.socket).LocalAddr().String(), (*socket.socket).RemoteAddr().String()
}

func NewSSHSocket(addr string, port int, sshPort int, sshUser string, sshPassword string) (Socket, error) {
	socket := &SSHSocket{}
	serverAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", addr, sshPort))
	if err != nil {
		return nil, err
	}
	socket.sshClient, err = ssh.Dial("tcp4", serverAddr.String(), &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return nil, err
	}

	serverAddr, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	socket1, err := socket.sshClient.Dial("tcp4", serverAddr.String())
	if err != nil {
		return nil, err
	}
	socket.socket = &socket1
	return socket, nil
}

func NewSSHListener(addr string, port int, network string) (Listener, error) {
	return NewTCPListener(addr, port, network)
}
