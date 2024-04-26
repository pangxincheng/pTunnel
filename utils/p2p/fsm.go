package p2p

import (
	"fmt"
	"net"
	"pTunnel/conn"
	"pTunnel/utils/log"
	"strings"
	"time"
)

type SocketWrapper struct {
	laddr       *net.UDPAddr
	raddr       *net.UDPAddr
	kcpSocket   *conn.KCPSocket
	kcpListener *conn.KCPListener
	udpSocket   *net.UDPConn
	cache       []byte
}

type FSM struct {
	socket    *SocketWrapper
	states    map[int]func(socket *SocketWrapper) int
	statesStr map[int]string
}

func NewFSM(socket *SocketWrapper) *FSM {
	return &FSM{
		states:    make(map[int]func(socket *SocketWrapper) int),
		statesStr: make(map[int]string),
		socket:    socket,
	}
}

func (fsm *FSM) AddState(state int, stateStr string, action func(socket *SocketWrapper) int) {
	fsm.states[state] = action
	fsm.statesStr[state] = stateStr
}

func (fsm *FSM) Run(state int) int {
	for {
		action, ok := fsm.states[state]
		if !ok {
			break
		}
		log.Info("State: %s", fsm.statesStr[state])
		state = action(fsm.socket)
	}
	return state
}

type FsmFn func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM

var fsmType2Func = map[string]FsmFn{
	"Fn10": func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM {
		v := NewFSM(&SocketWrapper{
			laddr: laddr,
			raddr: raddr,
			cache: make([]byte, 1024),
		})
		udpSocket, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			fmt.Printf("Failed to dial UDP. Error: %v\n", err)
			return nil
		}
		v.socket.udpSocket = udpSocket
		const (
			STOP = iota
			START
			SEND_SYN2
			CREATE_KCP_LISTENER
			SEND_HEARTBEAT
			ERR_STOP = -1
		)
		v.AddState(START, "START", func(socket *SocketWrapper) int {
			for {
				n, _, err := socket.udpSocket.ReadFromUDP(socket.cache)
				if err != nil {
					fmt.Printf("Failed to read from the user. Error: %v\n", err)
					return ERR_STOP
				}
				if string(socket.cache[:n]) == "SYN1" {
					break
				}
			}
			return SEND_SYN2
		})
		v.AddState(SEND_SYN2, "SEND_SYN2", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN2"))
			socket.udpSocket.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := socket.udpSocket.ReadFromUDP(socket.cache)
			if err != nil {
				fmt.Printf("Failed to read from the user. Error: %v\n", err)
				return SEND_SYN2
			}
			if string(socket.cache[:n]) == "SYN3" {
				socket.udpSocket.Close()
				socket.udpSocket = nil
				return CREATE_KCP_LISTENER
			}
			return SEND_SYN2
		})
		v.AddState(CREATE_KCP_LISTENER, "CREATE_KCP_LISTENER", func(socket *SocketWrapper) int {
			listener, err := conn.NewKCPListener(socket.laddr.IP.String(), socket.laddr.Port)
			if err != nil {
				fmt.Printf("Failed to create KCP listener. Error: %v\n", err)
				return ERR_STOP
			}
			socket.kcpListener = listener.(*conn.KCPListener)
			socket.kcpListener.Listener().SetDeadline(time.Now().Add(2 * time.Second))
			kcpSocket, err := socket.kcpListener.AcceptKCP()
			if err != nil {
				fmt.Printf("Failed to accept KCP connection. Error: %v\n", err)
				_ = socket.kcpListener.Close()
				socket.kcpListener = nil
				return ERR_STOP
			}
			socket.kcpSocket = kcpSocket
			bytes, _ := socket.kcpSocket.ReadLine()
			log.Info("Received: %s", string(bytes))
			return SEND_HEARTBEAT
		})
		v.AddState(SEND_HEARTBEAT, "SEND_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.WriteLine([]byte("HEARTBEAT"))
			return STOP
		})
		return v
	},
	"Fn11": func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM {
		v := NewFSM(&SocketWrapper{
			laddr: laddr,
			raddr: raddr,
			cache: make([]byte, 1024),
		})
		udpSocket, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			fmt.Printf("Failed to dial UDP. Error: %v\n", err)
			return nil
		}
		v.socket.udpSocket = udpSocket
		const (
			STOP = iota
			START
			SEND_SYN1
			SEND_SYN3
			CREATE_KCP_SOCKET
			KILL_KCP_SOCKET
			SEND_HEARTBEAT
			RECV_HEARTBEAT
			ERR_STOP = -1
		)
		v.AddState(START, "START", func(socket *SocketWrapper) int {
			return SEND_SYN1
		})
		v.AddState(SEND_SYN1, "SEND_SYN1", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN1"))
			socket.udpSocket.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := socket.udpSocket.ReadFromUDP(socket.cache)
			if err != nil {
				fmt.Printf("Failed to read from the user. Error: %v\n", err)
				return SEND_SYN1
			}
			if string(socket.cache[:n]) == "SYN2" {
				return SEND_SYN3
			}
			return SEND_SYN1
		})
		v.AddState(SEND_SYN3, "SEND_SYN3", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN3"))
			_ = socket.udpSocket.Close()
			socket.udpSocket = nil
			return CREATE_KCP_SOCKET
		})
		v.AddState(CREATE_KCP_SOCKET, "CREATE_KCP_SOCKET", func(socket *SocketWrapper) int {
			kcpSocket, err := conn.NewKCPSocketV2(socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to create KCP socket. Error: %v\n", err)
				return KILL_KCP_SOCKET
			}
			socket.kcpSocket = kcpSocket.(*conn.KCPSocket)
			return SEND_HEARTBEAT
		})
		v.AddState(KILL_KCP_SOCKET, "KILL_KCP_SOCKET", func(socket *SocketWrapper) int {
			_ = socket.kcpSocket.Close()
			socket.kcpSocket = nil
			socket.udpSocket, err = net.DialUDP("udp", socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to dial UDP. Error: %v\n", err)
				return ERR_STOP
			}
			return SEND_SYN3
		})
		v.AddState(SEND_HEARTBEAT, "SEND_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.WriteLine([]byte("HEARTBEAT"))
			return RECV_HEARTBEAT
		})
		v.AddState(RECV_HEARTBEAT, "RECV_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.GetSocket().SetReadDeadline(time.Now().Add(2 * time.Second))
			bytes, err := socket.kcpSocket.ReadLine()
			if err != nil {
				fmt.Printf("Failed to read line. Error: %v\n", err)
				return KILL_KCP_SOCKET
			}
			log.Info("Received: %s", string(bytes))
			return STOP
		})
		return v
	},
	"Fn20": func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM {
		v := NewFSM(&SocketWrapper{
			laddr: laddr,
			raddr: raddr,
			cache: make([]byte, 1024),
		})
		udpSocket, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			fmt.Printf("Failed to dial UDP. Error: %v\n", err)
			return nil
		}
		v.socket.udpSocket = udpSocket
		const (
			STOP = iota
			START
			SEND_SYN1
			SEND_SYN2
			CREATE_KCP_LISTENER
			KILL_KCP_LISTENER
			SEND_HEARTBEAT
			ERR_STOP = -1
		)
		v.AddState(START, "START", func(socket *SocketWrapper) int {
			return SEND_SYN1
		})
		v.AddState(SEND_SYN1, "SEND_SYN1", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN1"))
			socket.udpSocket.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := socket.udpSocket.ReadFromUDP(socket.cache)
			if err != nil {
				fmt.Printf("Failed to read from the user. Error: %v\n", err)
				return SEND_SYN1
			}
			if strings.HasPrefix(string(socket.cache[:n]), "SYN") {
				return SEND_SYN2
			}
			return SEND_SYN1
		})
		v.AddState(SEND_SYN2, "SEND_SYN2", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN2"))
			socket.udpSocket.Close()
			socket.udpSocket = nil
			return CREATE_KCP_LISTENER
		})
		v.AddState(CREATE_KCP_LISTENER, "CREATE_KCP_LISTENER", func(socket *SocketWrapper) int {
			listener, err := conn.NewKCPListener(socket.laddr.IP.String(), socket.laddr.Port)
			if err != nil {
				fmt.Printf("Failed to create KCP listener. Error: %v\n", err)
				return KILL_KCP_LISTENER
			}
			socket.kcpListener = listener.(*conn.KCPListener)
			socket.kcpListener.Listener().SetDeadline(time.Now().Add(2 * time.Second))
			kcpSocket, err := socket.kcpListener.AcceptKCP()
			if err != nil {
				fmt.Printf("Failed to accept KCP connection. Error: %v\n", err)
				return KILL_KCP_LISTENER
			}
			socket.kcpSocket = kcpSocket
			bytes, _ := socket.kcpSocket.ReadLine()
			log.Info("Received: %s", string(bytes))
			return SEND_HEARTBEAT
		})
		v.AddState(KILL_KCP_LISTENER, "KILL_KCP_LISTENER", func(socket *SocketWrapper) int {
			_ = socket.kcpListener.Close()
			socket.kcpListener = nil
			time.Sleep(1 * time.Second)
			socket.udpSocket, err = net.DialUDP("udp", socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to dial UDP. Error: %v\n", err)
				return ERR_STOP
			}
			return SEND_SYN2
		})
		v.AddState(SEND_HEARTBEAT, "SEND_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.WriteLine([]byte("HEARTBEAT"))
			return STOP
		})
		return v
	},
	"Fn21": func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM {
		v := NewFSM(&SocketWrapper{
			laddr: laddr,
			raddr: raddr,
			cache: make([]byte, 1024),
		})
		udpSocket, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			fmt.Printf("Failed to dial UDP. Error: %v\n", err)
			return nil
		}
		v.socket.udpSocket = udpSocket
		const (
			STOP = iota
			START
			SEND_SYN1
			SEND_SYN2
			CREATE_KCP_SOCKET
			KILL_KCP_SOCKET
			SEND_HEARTBEAT
			RECV_HEARTBEAT
			ERR_STOP = -1
		)
		v.AddState(START, "START", func(socket *SocketWrapper) int {
			return SEND_SYN1
		})
		v.AddState(SEND_SYN1, "SEND_SYN1", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN1"))
			socket.udpSocket.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := socket.udpSocket.ReadFromUDP(socket.cache)
			if err != nil {
				fmt.Printf("Failed to read from the user. Error: %v\n", err)
				return SEND_SYN1
			}
			if strings.HasPrefix(string(socket.cache[:n]), "SYN") {
				return SEND_SYN2
			}
			return SEND_SYN1
		})
		v.AddState(SEND_SYN2, "SEND_SYN2", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN2"))
			socket.udpSocket.Close()
			socket.udpSocket = nil
			return CREATE_KCP_SOCKET
		})
		v.AddState(CREATE_KCP_SOCKET, "CREATE_KCP_SOCKET", func(socket *SocketWrapper) int {
			kcpSocket, err := conn.NewKCPSocketV2(socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to create KCP socket. Error: %v\n", err)
				return KILL_KCP_SOCKET
			}
			socket.kcpSocket = kcpSocket.(*conn.KCPSocket)
			return SEND_HEARTBEAT
		})
		v.AddState(KILL_KCP_SOCKET, "KILL_KCP_SOCKET", func(socket *SocketWrapper) int {
			_ = socket.kcpSocket.Close()
			socket.kcpSocket = nil
			time.Sleep(1 * time.Second)
			socket.udpSocket, err = net.DialUDP("udp", socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to dial UDP. Error: %v\n", err)
				return ERR_STOP
			}
			return SEND_SYN2
		})
		v.AddState(SEND_HEARTBEAT, "SEND_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.WriteLine([]byte("HEARTBEAT"))
			return RECV_HEARTBEAT
		})
		v.AddState(RECV_HEARTBEAT, "RECV_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.GetSocket().SetReadDeadline(time.Now().Add(2 * time.Second))
			bytes, err := socket.kcpSocket.ReadLine()
			if err != nil {
				fmt.Printf("Failed to read line. Error: %v\n", err)
				return KILL_KCP_SOCKET
			}
			log.Info("Received: %s", string(bytes))
			return STOP
		})
		return v
	},
	"Fn30": func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM {
		v := NewFSM(&SocketWrapper{
			laddr: laddr,
			raddr: raddr,
			cache: make([]byte, 1024),
		})
		udpSocket, err := net.ListenUDP("udp", laddr)
		if err != nil {
			fmt.Printf("Failed to dial UDP. Error: %v\n", err)
			return nil
		}
		v.socket.udpSocket = udpSocket
		const (
			STOP = iota
			START
			SEND_SYN1
			SEND_SYN2
			CREATE_KCP_LISTENER
			KILL_KCP_LISTENER
			SEND_HEARTBEAT
			ERR_STOP = -1
		)
		v.AddState(START, "START", func(socket *SocketWrapper) int {
			return SEND_SYN1
		})
		v.AddState(SEND_SYN1, "SEND_SYN1", func(socket *SocketWrapper) int {
			// TODO: Port prediction attack

			ports := make([]int, 65535-1024)
			for i := 1024; i < 65535; i++ {
				ports[i-1024] = i
			}
			// rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(ports), func(i, j int) { ports[i], ports[j] = ports[j], ports[i] })
			for _, port := range ports {
				log.Info("Trying port: %d", port)
				socket.udpSocket.WriteToUDP([]byte("SYN1"), &net.UDPAddr{
					IP:   raddr.IP,
					Port: port,
				})
				if port == socket.raddr.Port {
					fmt.Printf("Found the port: %d\n", port)
				}
				time.Sleep(1 * time.Millisecond)
			}
			socket.udpSocket.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, addr, err := socket.udpSocket.ReadFromUDP(socket.cache)
			if err != nil {
				fmt.Printf("Failed to read from the user. Error: %v\n", err)
				return SEND_SYN1
			}
			if strings.HasPrefix(string(socket.cache[:n]), "SYN") {
				socket.raddr = addr // Update the remote address
				return SEND_SYN2
			}
			return SEND_SYN1
		})
		v.AddState(SEND_SYN2, "SEND_SYN2", func(socket *SocketWrapper) int {
			socket.udpSocket.WriteToUDP([]byte("SYN2"), socket.raddr)
			socket.udpSocket.Close()
			socket.udpSocket = nil
			return CREATE_KCP_LISTENER
		})
		v.AddState(CREATE_KCP_LISTENER, "CREATE_KCP_LISTENER", func(socket *SocketWrapper) int {
			listener, err := conn.NewKCPListener(socket.laddr.IP.String(), socket.laddr.Port)
			if err != nil {
				fmt.Printf("Failed to create KCP listener. Error: %v\n", err)
				return KILL_KCP_LISTENER
			}
			socket.kcpListener = listener.(*conn.KCPListener)
			socket.kcpListener.Listener().SetDeadline(time.Now().Add(2 * time.Second))
			kcpSocket, err := socket.kcpListener.AcceptKCP()
			if err != nil {
				fmt.Printf("Failed to accept KCP connection. Error: %v\n", err)
				return KILL_KCP_LISTENER
			}
			socket.kcpSocket = kcpSocket
			bytes, _ := socket.kcpSocket.ReadLine()
			log.Info("Received: %s", string(bytes))
			return SEND_HEARTBEAT
		})
		v.AddState(KILL_KCP_LISTENER, "KILL_KCP_LISTENER", func(socket *SocketWrapper) int {
			_ = socket.kcpListener.Close()
			socket.kcpListener = nil
			time.Sleep(1 * time.Second)
			socket.udpSocket, err = net.ListenUDP("udp", laddr)
			if err != nil {
				fmt.Printf("Failed to dial UDP. Error: %v\n", err)
				return ERR_STOP
			}
			return SEND_SYN2
		})
		v.AddState(SEND_HEARTBEAT, "SEND_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.WriteLine([]byte("HEARTBEAT"))
			return STOP
		})
		return v
	},
	"Fn31": func(laddr *net.UDPAddr, raddr *net.UDPAddr) *FSM {
		v := NewFSM(&SocketWrapper{
			laddr: laddr,
			raddr: raddr,
			cache: make([]byte, 1024),
		})
		udpSocket, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			fmt.Printf("Failed to dial UDP. Error: %v\n", err)
			return nil
		}
		v.socket.udpSocket = udpSocket
		const (
			STOP = iota
			START
			SEND_SYN1
			SEND_SYN2
			CREATE_KCP_SOCKET
			KILL_KCP_SOCKET
			SEND_HEARTBEAT
			RECV_HEARTBEAT
			ERR_STOP = -1
		)
		v.AddState(START, "START", func(socket *SocketWrapper) int {
			return SEND_SYN1
		})
		v.AddState(SEND_SYN1, "SEND_SYN1", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN1"))
			socket.udpSocket.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := socket.udpSocket.ReadFromUDP(socket.cache)
			if err != nil {
				fmt.Printf("Failed to read from the user. Error: %v\n", err)
				return SEND_SYN1
			}
			if strings.HasPrefix(string(socket.cache[:n]), "SYN") {
				return SEND_SYN2
			}
			return SEND_SYN1
		})
		v.AddState(SEND_SYN2, "SEND_SYN2", func(socket *SocketWrapper) int {
			socket.udpSocket.Write([]byte("SYN2"))
			socket.udpSocket.Close()
			socket.udpSocket = nil
			return CREATE_KCP_SOCKET
		})
		v.AddState(CREATE_KCP_SOCKET, "CREATE_KCP_SOCKET", func(socket *SocketWrapper) int {
			kcpSocket, err := conn.NewKCPSocketV2(socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to create KCP socket. Error: %v\n", err)
				return KILL_KCP_SOCKET
			}
			socket.kcpSocket = kcpSocket.(*conn.KCPSocket)
			return SEND_HEARTBEAT
		})
		v.AddState(KILL_KCP_SOCKET, "KILL_KCP_SOCKET", func(socket *SocketWrapper) int {
			_ = socket.kcpSocket.Close()
			socket.kcpSocket = nil
			time.Sleep(1 * time.Second)
			socket.udpSocket, err = net.DialUDP("udp", socket.laddr, socket.raddr)
			if err != nil {
				fmt.Printf("Failed to dial UDP. Error: %v\n", err)
				return ERR_STOP
			}
			return SEND_SYN2
		})
		v.AddState(SEND_HEARTBEAT, "SEND_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.WriteLine([]byte("HEARTBEAT"))
			return RECV_HEARTBEAT
		})
		v.AddState(RECV_HEARTBEAT, "RECV_HEARTBEAT", func(socket *SocketWrapper) int {
			socket.kcpSocket.GetSocket().SetReadDeadline(time.Now().Add(2 * time.Second))
			bytes, err := socket.kcpSocket.ReadLine()
			if err != nil {
				fmt.Printf("Failed to read line. Error: %v\n", err)
				return KILL_KCP_SOCKET
			}
			log.Info("Received: %s", string(bytes))
			return STOP
		})
		return v
	},
}

func GetFSM(fsmType string) FsmFn {
	fn, ok := fsmType2Func[fsmType]
	if ok {
		return fn
	}
	return nil
}
