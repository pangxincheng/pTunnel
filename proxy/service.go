package proxy

import (
	"errors"
	"fmt"
	"net"
	"pTunnel/conn"
	tunnel2 "pTunnel/tunnel"
	"pTunnel/utils/consts"
	"pTunnel/utils/log"
	"pTunnel/utils/p2p"
	"pTunnel/utils/security"
	"pTunnel/utils/serialize"
	"strconv"
	"sync"
)

type Service struct {
	Name       string // set mannually
	ProxyPort  int    // set mannually
	ProxyType  string // set mannually
	TunnelPort int    // set mannually
	TunnelType string // set mannually
	P2PAddr    string // set automatically
	P2PAddrV4  string // only for p2p tunnel, optional
	P2PAddrV6  string // only for p2p tunnel, optional
	P2PPort    int    // only for p2p tunnel, optional
	P2PType    string // only for p2p tunnel, optional

	ProxySocket  conn.Socket // set automatically
	TunnelSocket conn.Socket // set automatically

	// Metadata
	LAddr         *net.UDPAddr // set automatically
	RAddr         *net.UDPAddr // set automatically
	FSMType       string       // set automatically
	SecretKey     []byte       // set automatically
	TunnelEncrypt bool         // set automatically
}

func (service *Service) run() {
	defer service.ProxySocket.Close()
	defer service.closeTunnelSocket()

	// Create tunnel socket
	if service.createTunnelSocket() != nil {
		return
	}

	// Extract metadata
	if service.extractMetadata() != nil {
		return
	}

	// Close tunnel socket after metadata exchange
	service.closeTunnelSocket() // Close tunnel socket after metadata exchange

	// UDP hole punching
	if service.udpHolePunching() != nil {
		return
	}

	// Tunnel
	service.tunnel()

}

func (service *Service) createTunnelSocket() (err error) {
	socketType := "kcp4"
	if service.TunnelType == "p2p6" {
		socketType = "kcp6"
	}
	tunnelSocket, err := conn.NewSocket(
		socketType,
		consts.Auto, consts.Auto, 0,
		ServerAddrV4, ServerAddrV6,
		service.TunnelPort, consts.UnConf, 0, nil,
	)
	if err != nil {
		log.Error("Create tunnel socket failed. Error: %v", err)
		return
	}
	service.TunnelSocket = tunnelSocket
	return
}

func (service *Service) extractMetadata() (err error) {
	secretKey := security.AesGenKey(32) // only for encrypt/decrypt metadata
	dict := make(map[string]interface{})
	if service.P2PAddrV4 != "" {
		service.P2PAddr = service.P2PAddrV4
		if !conn.IsValidIP(service.P2PAddrV4) {
			service.P2PAddr, err = conn.GetIPAddressFromInterfaceName(service.P2PAddrV4, "ipv4")
			if err != nil {
				log.Error("Service [%s] get IP address failed. Error: %v", service.Name, err)
				return
			}
		}
		service.P2PPort, err = conn.GetAvailablePort("udp4")
		if err != nil {
			log.Error("Get available port failed. Error: %v", err)
			return
		}
		dict["Addr"] = service.P2PAddr
		dict["Port"] = strconv.Itoa(service.P2PPort)
		dict["Network"] = "udp4"
	} else if service.P2PAddrV6 != "" {
		service.P2PAddr = service.P2PAddrV6
		if !conn.IsValidIP(service.P2PAddrV6) {
			service.P2PAddr, err = conn.GetIPAddressFromInterfaceName(service.P2PAddrV6, "ipv6")
			if err != nil {
				log.Error("Service [%s] get IP address failed. Error: %v", service.Name, err)
				return
			}
		}
		service.P2PPort, err = conn.GetAvailablePort("udp6")
		if err != nil {
			log.Error("Get available port failed. Error: %v", err)
			return
		}
		dict["Addr"] = service.P2PAddr
		dict["Port"] = strconv.Itoa(service.P2PPort)
		dict["Network"] = "udp6"
	}
	dict["Type"] = "Proxy"
	dict["NATType"] = strconv.Itoa(MappingType*3 + FilteringType)
	dict["SecretKey"] = string(secretKey)
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
	err = service.TunnelSocket.WriteLine(bytes)
	if err != nil {
		log.Error("Send metadata failed. Error: %v", err)
		return
	}

	bytes, err = service.TunnelSocket.ReadLine()
	if err != nil {
		log.Error("Receive metadata failed. Error: %v", err)
		return
	}

	bytes, err = security.AESDecryptBase64(bytes, secretKey)
	if err != nil {
		log.Error("Decrypt metadata failed. Error: %v", err)
		return
	}

	dict = make(map[string]interface{})
	err = serialize.Deserialize(bytes, &dict)
	if err != nil {
		log.Error("Deserialize metadata failed. Error: %v", err)
		return
	}

	service.RAddr, err = net.ResolveUDPAddr(dict["RNetwork"].(string), fmt.Sprintf("%s:%s", dict["RAddr"].(string), dict["RPort"].(string)))
	if err != nil {
		log.Error("Resolve remote address failed. Error: %v", err)
		return
	}
	if service.P2PAddrV4 != "" {
		service.LAddr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", service.P2PAddr, strconv.Itoa(service.P2PPort)))
	} else if service.P2PAddrV6 != "" {
		service.LAddr, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%s", service.P2PAddr, strconv.Itoa(service.P2PPort)))
	} else if service.TunnelType == "p2p4" {
		service.LAddr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", "0.0.0.0", strconv.Itoa(service.TunnelPort)))
	} else if service.TunnelType == "p2p6" {
		service.LAddr, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%s", "[::]", strconv.Itoa(service.TunnelPort)))
	}
	if err != nil {
		log.Error("Resolve local address failed. Error: %v", err)
		return
	}
	service.FSMType = dict["FSMType"].(string)
	service.SecretKey = []byte(dict["SecretKey"].(string)) // tunnel secret key
	return
}

func (service *Service) closeTunnelSocket() {
	if service.TunnelSocket != nil {
		service.TunnelSocket.Close()
	}
	service.TunnelSocket = nil
}

func (service *Service) udpHolePunching() (err error) {
	fsmFn := p2p.GetFSM(service.FSMType)
	if fsmFn == nil {
		log.Error("Unsupported FSM type: %s", service.FSMType)
		err = errors.New("unsupported FSM type")
		return
	}
	fsm := fsmFn(service.LAddr, service.RAddr)
	if fsm == nil {
		log.Error("Create FSM failed")
		err = errors.New("create FSM failed")
		return
	}
	if fsm.Run(1) != 0 {
		log.Error("Run FSM failed")
		err = errors.New("run FSM failed")
		return
	}
	log.Info("UDP hole punching success")
	service.TunnelSocket = fsm.GetKCPSocket()
	return
}

func (service *Service) tunnel() {
	tunnel := service.TunnelSocket
	proxy := service.ProxySocket
	closeFn := func(tunnel conn.Socket) {
		err := tunnel.Close()
		if err != nil {
			log.Error("Close a tunnel failed. Error: %v", err)
		}
	}
	defer closeFn(tunnel)
	defer closeFn(proxy)
	if !tunnel2.ClientTunnelSafetyCheck(tunnel, service.SecretKey) {
		log.Error("Tunnel safety check failed")
		return
	}
	if !service.TunnelEncrypt {
		tunnel2.UnsafeTunnel(proxy, tunnel)
		return
	} else {
		tunnel2.SafeTunnel(proxy, tunnel, service.SecretKey)
	}
}

var services = make(map[string]*Service)

func RegisterService(
	name string,
	proxyPort int,
	proxyType string,
	tunnelPort int,
	tunnelType string,
	p2pAddrV4 string,
	p2pAddrV6 string,
) {
	if _, ok := services[name]; ok {
		panic("service already exists")
	}
	services[name] = &Service{
		Name:       name,
		ProxyPort:  proxyPort,
		ProxyType:  proxyType,
		TunnelPort: tunnelPort,
		TunnelType: tunnelType,
		P2PAddrV4:  p2pAddrV4,
		P2PAddrV6:  p2pAddrV6,
	}
}

func Run() {
	log.InitLog(LogWay, LogFile, LogLevel, LogMaxDays)
	var wait sync.WaitGroup
	wait.Add(len(services))
	for _, service := range services {
		go func(service *Service) {
			defer wait.Done()
			listener, err := conn.NewListener(service.ProxyType, consts.Auto, service.ProxyPort)
			if err != nil {
				log.Error("Create proxy listener failed. Error: %v", err)
				return
			}
			for {
				socket, err := listener.Accept()
				if err != nil {
					log.Error("Accept connection failed. Error: %v", err)
					continue
				}
				service.ProxySocket = socket
				go service.run()
			}
		}(service)
	}
	wait.Wait()
}
