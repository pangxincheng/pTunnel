package conn

import (
	"errors"
	"fmt"
	"net"
	"pTunnel/utils/consts"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func NewListener(lType string, ip string, port int) (Listener, error) {
	var listener Listener
	switch strings.ToLower(lType) {
	case "tcp4":
		if ip == consts.Auto {
			ip = "0.0.0.0"
		}
		addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return nil, err
		}
		listener, err = NewTCPListener(addr, "tcp4")
		if err != nil {
			return nil, err
		}
	case "tcp6":
		if ip == consts.Auto {
			ip = "[::]"
		}
		addr, err := net.ResolveTCPAddr("tcp6", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return nil, err
		}
		listener, err = NewTCPListener(addr, "tcp6")
		if err != nil {
			return nil, err
		}
	case "kcp4":
		if ip == consts.Auto {
			ip = "0.0.0.0"
		}
		addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return nil, err
		}
		listener, err = NewKCPListener(addr)
		if err != nil {
			return nil, err
		}
	case "kcp6":
		if ip == consts.Auto {
			ip = "[::]"
		}
		addr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return nil, err
		}
		listener, err = NewKCPListener(addr)
		if err != nil {
			return nil, err
		}
	case "ssh4":
		if ip == consts.Auto {
			ip = "0.0.0.0"
		}
		addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return nil, err
		}
		listener, err = NewSSHListener(addr, "tcp4")
		if err != nil {
			return nil, err
		}
	case "ssh6":
		if ip == consts.Auto {
			ip = "[::]"
		}
		addr, err := net.ResolveTCPAddr("tcp6", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return nil, err
		}
		listener, err = NewSSHListener(addr, "tcp6")
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported listener type: " + lType)
	}
	return listener, nil
}

func NewSocket(sType string, lip4 string, lip6 string, lport int, rip4 string, rip6 string, rport int, sshUser string, sshSigher ssh.Signer) (Socket, error) {
	var socket Socket
	switch strings.ToLower(sType) {
	case "tcp4":
		var laddr4 *net.TCPAddr
		var raddr4 *net.TCPAddr
		var err error
		if lip4 == consts.Auto {
			laddr4 = nil
		} else {
			laddr4, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", lip4, lport))
			if err != nil {
				return nil, err
			}
		}
		raddr4, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", rip4, rport))
		if err != nil {
			return nil, err
		}
		socket, err = NewTCPSocket(laddr4, raddr4, "tcp4")
		if err != nil {
			return nil, err
		}
	case "tcp6":
		var laddr6 *net.TCPAddr
		var raddr6 *net.TCPAddr
		var err error
		if lip6 == consts.Auto {
			laddr6 = nil
		} else {
			laddr6, err = net.ResolveTCPAddr("tcp6", fmt.Sprintf("%s:%d", lip6, lport))
			if err != nil {
				return nil, err
			}
		}
		raddr6, err = net.ResolveTCPAddr("tcp6", fmt.Sprintf("%s:%d", rip6, rport))
		if err != nil {
			return nil, err
		}
		socket, err = NewTCPSocket(laddr6, raddr6, "tcp6")
		if err != nil {
			return nil, err
		}
	case "kcp4":
		var laddr4 *net.UDPAddr
		var raddr4 *net.UDPAddr
		var err error
		if lip4 == consts.Auto {
			laddr4 = nil
		} else {
			laddr4, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", lip4, lport))
			if err != nil {
				return nil, err
			}
		}
		raddr4, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", rip4, rport))
		if err != nil {
			return nil, err
		}
		socket, err = NewKCPSocket(laddr4, raddr4, "udp4")
		if err != nil {
			return nil, err
		}
	case "kcp6":
		var laddr6 *net.UDPAddr
		var raddr6 *net.UDPAddr
		var err error
		if lip6 == consts.Auto {
			laddr6 = nil
		} else {
			laddr6, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%d", lip6, lport))
			if err != nil {
				return nil, err
			}
		}
		raddr6, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%d", rip6, rport))
		if err != nil {
			return nil, err
		}
		socket, err = NewKCPSocket(laddr6, raddr6, "udp6")
		if err != nil {
			return nil, err
		}
	case "ssh4":
		var raddr4 *net.TCPAddr
		var err error
		raddr4, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", rip4, rport))
		if err != nil {
			return nil, err
		}
		socket, err = NewSSHSocket(raddr4, "tcp4", rport, sshUser, sshSigher)
		if err != nil {
			return nil, err
		}
	case "ssh6":
		var raddr6 *net.TCPAddr
		var err error
		raddr6, err = net.ResolveTCPAddr("tcp6", fmt.Sprintf("%s:%d", rip6, rport))
		if err != nil {
			return nil, err
		}
		socket, err = NewSSHSocket(raddr6, "tcp6", rport, sshUser, sshSigher)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported socket type: " + sType)
	}
	return socket, nil
}

func GetAvailablePort(lType string) (port int, err error) {
	switch strings.ToLower(lType) {
	case "tcp4":
		var addr *net.TCPAddr
		var listener *net.TCPListener
		addr, err = net.ResolveTCPAddr("tcp4", "0.0.0.0:0")
		if err != nil {
			return
		}
		listener, err = net.ListenTCP("tcp4", addr)
		if err != nil {
			return
		}
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		time.Sleep(100 * time.Millisecond)
	case "tcp6":
		var addr *net.TCPAddr
		var listener *net.TCPListener
		addr, err = net.ResolveTCPAddr("tcp6", "[::]:0")
		if err != nil {
			return
		}
		listener, err = net.ListenTCP("tcp6", addr)
		if err != nil {
			return
		}
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		time.Sleep(100 * time.Millisecond)
	case "udp4":
		var addr *net.UDPAddr
		var listener *net.UDPConn
		addr, err = net.ResolveUDPAddr("udp4", "0.0.0.0:0")
		if err != nil {
			return
		}
		listener, err = net.ListenUDP("udp4", addr)
		if err != nil {
			return
		}
		port = listener.LocalAddr().(*net.UDPAddr).Port
		listener.Close()
		time.Sleep(100 * time.Millisecond)
	case "udp6":
		var addr *net.UDPAddr
		var listener *net.UDPConn
		addr, err = net.ResolveUDPAddr("udp6", "[::]:0")
		if err != nil {
			return
		}
		listener, err = net.ListenUDP("udp6", addr)
		if err != nil {
			return
		}
		port = listener.LocalAddr().(*net.UDPAddr).Port
		listener.Close()
		time.Sleep(100 * time.Millisecond)
	default:
		err = errors.New("unsupported listener type: " + lType)
	}
	return
}
