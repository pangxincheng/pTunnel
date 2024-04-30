package proxy

import (
	"fmt"
	"net"
	"pTunnel/conn"
	tunnel2 "pTunnel/tunnel"
	"pTunnel/utils/log"
	"pTunnel/utils/p2p"
	"pTunnel/utils/security"
	"pTunnel/utils/serialize"
	"strconv"
	"strings"
)

type Proxy struct {
	proxySocket *conn.TCPSocket
	socket      *conn.KCPSocket
}

func (proxy *Proxy) run() {
	log.Info("Proxy start running...")

	var mappingType int
	var filteringType int
	var err error
	if NATType != -1 {
		log.Info("Configured NAT type manually")
		mappingType = NATType / 3
		filteringType = NATType % 3
	} else {
		log.Info("Start to check NAT type automatically")
		mappingType, filteringType, err = p2p.CheckNATType("stun.miwifi.com:3478", 5)
		if err != nil {
			log.Error("Failed to check NAT type. Error: %v", err)
			return
		}
	}
	log.Info("NAT type is set to %d(mappingType=%d, filteringType=%d)", mappingType*3+filteringType, mappingType, filteringType)

	// Construct metadata, serialize and encrypt
	dict := make(map[string]interface{})
	dict["SecretKey"] = string(security.AesGenKey(32))
	if P2pAddr != "" {
		dict["Addr"] = P2pAddr
	}
	dict["Type"] = "Proxy"
	dict["NATType"] = strconv.Itoa(mappingType*3 + filteringType)
	bytes, err := serialize.Serialize(&dict)
	if err != nil {
		log.Error("Serialize metadata failed. Error: %v", err)
		return
	}
	bytes, err = security.RSAEncryptBase64(bytes, PublicKey, NBits)
	if err != nil {
		log.Error("Encrypt metadata failed. Error: %v", err)
		return
	}
	err = proxy.socket.WriteLine(bytes)
	if err != nil {
		log.Error("Send metadata to server failed. Error: %v", err)
		return
	}

	bytes, err = proxy.socket.ReadLine()
	if err != nil {
		log.Error("Receive response from server failed. Error: %v", err)
		return
	}
	bytes, err = security.AESDecryptBase64(bytes, []byte(dict["SecretKey"].(string)))
	if err != nil {
		log.Error("Decrypt response from server failed. Error: %v", err)
		return
	}
	fmt.Println(string(bytes))

	odict := make(map[string]interface{})
	err = serialize.Deserialize(bytes, &odict)
	if err != nil {
		log.Error("Deserialize response from server failed. Error: %v", err)
		return
	}

	// UDP hole punching
	_ = proxy.socket.Close()
	var laddr *net.UDPAddr
	if P2pAddr != "" {
		laddr, err = net.ResolveUDPAddr("udp", P2pAddr)
	} else {
		laddr, err = net.ResolveUDPAddr("udp", proxy.socket.GetSocket().LocalAddr().String())
	}
	if err != nil {
		log.Error("Resolve local UDP address failed. Error: %v", err)
		return
	}
	raddr, err := net.ResolveUDPAddr("udp", odict["Addr"].(string))
	if err != nil {
		log.Error("Resolve remote UDP address failed. Error: %v", err)
		return
	}
	fmt.Println("laddr: ", laddr, ", raddr: ", raddr)
	fsmFn := p2p.GetFSM(odict["FSM"].(string))
	if fsmFn == nil {
		log.Error("Failed to get FSM function")
		return
	}
	fsm := fsmFn(laddr, raddr)
	if fsm == nil {
		log.Error("Failed to create FSM")
		return
	}
	if fsm.Run(1) != 0 {
		log.Error("Failed to run FSM")
		return
	}
	fmt.Println("UDP hole punching done")
	kcpSocket := fsm.GetKCPSocket()
	proxy.tunnel(proxy.proxySocket, kcpSocket)
}

func (proxy *Proxy) tunnel(client conn.Socket, tunnel conn.Socket) {
	closeFn := func(tunnel conn.Socket) {
		err := tunnel.Close()
		if err != nil {
			log.Error("Close a tunnel failed. Error: %v", err)
		}
	}
	defer closeFn(tunnel)
	defer closeFn(client)
	// if !tunnel2.ClientTunnelSafetyCheck(tunnel, proxy.SecretKey) {
	// 	log.Error("Tunnel safety check failed")
	// 	return
	// }
	// if !proxy.TunnelEncrypt {
	tunnel2.UnsafeTunnel(client, tunnel)
	// 	return
	// } else {
	// 	tunnel2.SafeTunnel(client, tunnel, proxy.SecretKey)
	// }
}

func Run() {
	log.InitLog(LogWay, LogFile, LogLevel, LogMaxDays)

	var addr *net.TCPAddr
	var err error
	switch strings.ToLower(LocalType) {
	case "tcp", "tcp4":
		addr, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("0.0.0.0:%d", LocalPort))
	case "tcp6":
		addr, err = net.ResolveTCPAddr("tcp6", fmt.Sprintf("[::]:%d", LocalPort))
	default:
		log.Error("Unsupported local type: %s", LocalType)
		return
	}
	if err != nil {
		log.Error("Failed to resolve TCP address. Error: %v", err)
		return
	}

	proxyServer, err := conn.NewTCPListenerV2(addr)
	if err != nil {
		log.Error("Failed to create TCP Listener. Error: %v", err)
		return
	}

	for {
		socket, err := proxyServer.Accept()
		if err != nil {
			log.Error("Failed to accept connection. Error: %v", err)
			continue
		}
		proxy := &Proxy{}
		kcpSocket, err := conn.NewKCPSocket(ServerAddr, ServerPort, "udp")
		if err != nil {
			log.Error("Failed to create KCP Socket. Error: %v", err)
			continue
		}
		proxy.socket = kcpSocket.(*conn.KCPSocket)
		proxy.proxySocket = socket.(*conn.TCPSocket)
		go proxy.run()
	}
}
