package conn

import (
	"bufio"
	"net"

	"golang.org/x/crypto/ssh"
)

type SSHSocket struct {
	sshClient *ssh.Client
	Socket    *net.Conn
	reader    *bufio.Reader
}

func (socket *SSHSocket) Close() error {
	err := (*socket.Socket).Close()
	if err != nil {
		return err
	}
	return socket.sshClient.Close()
}

func (socket *SSHSocket) Write(p []byte) (n int, err error) {
	return (*socket.Socket).Write(p)
}

func (socket *SSHSocket) Read(p []byte) (n int, err error) {
	return (*socket.Socket).Read(p)
}
func (socket *SSHSocket) ReadLine() ([]byte, error) {
	return socket.reader.ReadBytes('\n')
}

func (socket *SSHSocket) WriteLine(bytes []byte) error {
	_, err := (*socket.Socket).Write(append(bytes, '\n'))
	return err
}

func (socket *SSHSocket) RemoteAddr() net.Addr {
	return (*socket.Socket).RemoteAddr()
}

func (socket *SSHSocket) LocalAddr() net.Addr {
	return (*socket.Socket).LocalAddr()
}

func (socket *SSHSocket) Address() (net.Addr, net.Addr) {
	return (*socket.Socket).LocalAddr(), (*socket.Socket).RemoteAddr()
}

func NewSSHSocket(raddr *net.TCPAddr, network string, sshAddr string, sshUser string, sshSigher ssh.Signer) (Socket, error) {
	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshSigher),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshClient, err := ssh.Dial(network, sshAddr, sshConfig)
	if err != nil {
		return nil, err
	}
	conn, err := sshClient.Dial(network, raddr.String())
	if err != nil {
		return nil, err
	}
	return &SSHSocket{
		sshClient: sshClient,
		Socket:    &conn,
		reader:    bufio.NewReader(conn.(ssh.Channel)),
	}, nil
}

func NewSSHListener(addr *net.TCPAddr, network string) (Listener, error) {
	return NewTCPListener(addr, network)
}
