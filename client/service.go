package client

import (
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
	"strings"
	"sync"
	"time"
)

type Service struct {
	Name             string
	SecretKey        []byte
	InternalAddr     string
	InternalPort     int
	InternalType     string
	ExternalPort     int
	ExternalType     string
	TunnelType       string
	TunnelPort       int
	TunnelEncrypt    bool
	HeartbeatTimeout int
	SshPort          int    // only for ssh tunnel
	SshUser          string // only for ssh tunnel
	SshPassword      string // only for ssh tunnel

	ControlSocket  conn.Socket
	ControlMsgChan chan int
	TunnelMsgChan  chan int
}

func (service *Service) run(wait *sync.WaitGroup) {
	defer wait.Done()
	log.Info("Service [%s] is running", service.Name)

	// Generate SecretKey
	service.SecretKey = security.AesGenKey(32)

	// Construct metadata, serialize and encrypt
	dict := make(map[string]interface{})
	dict["SecretKey"] = string(service.SecretKey)
	dict["ExternalPort"] = strconv.Itoa(service.ExternalPort)
	dict["ExternalType"] = service.ExternalType
	dict["TunnelType"] = service.TunnelType
	dict["TunnelEncrypt"] = service.TunnelEncrypt
	dict["TunnelPort"] = strconv.Itoa(service.TunnelPort)
	bytes, err := serialize.Serialize(&dict)
	if err != nil {
		log.Error("Service [%s] serialize metadata failed. Error: %v", service.Name, err)
		return
	}
	bytes, err = security.RSAEncryptBase64(bytes, PublicKey, NBits)
	if err != nil {
		log.Error("Service [%s] encrypt metadata failed. Error: %v", service.Name, err)
		return
	}

	// Connect to the server
	switch ServerType {
	case "tcp", "tcp4":
		log.Info("Service [%s] connect to server %s:%d", service.Name, ServerAddrV4, ServerPort)
		service.ControlSocket, err = conn.NewTCPSocket(ServerAddrV4, ServerPort, "tcp4")
	case "tcp6":
		log.Info("Service [%s] connect to server %s:%d", service.Name, ServerAddrV6, ServerPort)
		service.ControlSocket, err = conn.NewTCPSocket(ServerAddrV6, ServerPort, "tcp6")
	default:
		log.Error("Unknown server type: %s", ServerType)
		return
	}
	if err != nil {
		log.Error("Service [%s] connect to server failed. Error: %v", service.Name, err)
		return
	}

	// Send metadata to the server
	err = service.ControlSocket.WriteLine(bytes)
	if err != nil {
		log.Error("Service [%s] send metadata to server failed. Error: %v", service.Name, err)
		return
	}

	// Receive response from the server, decrypt and deserialize
	bytes, err = service.ControlSocket.ReadLine()
	if err != nil {
		log.Error("Service [%s] receive response from server failed. Error: %v", service.Name, err)
		return
	}
	bytes, err = security.AESDecryptBase64(bytes, service.SecretKey)
	if err != nil {
		log.Error("Service [%s] decrypt response from server failed. Error: %v", service.Name, err)
		return
	}
	dict = make(map[string]interface{})
	err = serialize.Deserialize(bytes, &dict)
	if err != nil {
		log.Error("Service [%s] deserialize response from server failed. Error: %v", service.Name, err)
		return
	}
	status, err := strconv.Atoi(dict["Status"].(string))
	if err != nil {
		log.Error("Service [%s] parse status from response failed. Error: %v", service.Name, err)
		return
	}
	if status != 200 {
		log.Error("Service [%s] response status is not 200. Status: %d", service.Name, status)
		return
	}
	service.TunnelPort, err = strconv.Atoi(dict["TunnelPort"].(string))
	if err != nil {
		log.Error("Service [%s] parse tunnel port from response failed. Error: %v", service.Name, err)
		return
	}
	service.HeartbeatTimeout, err = strconv.Atoi(dict["HeartbeatTimeout"].(string))
	if err != nil {
		log.Error("Service [%s] parse heartbeat timeout from response failed. Error: %v", service.Name, err)
		return
	}

	service.SshPort, err = strconv.Atoi(dict["SshPort"].(string))
	if err != nil {
		log.Error("Service [%s] parse ssh port from response failed. Error: %v", service.Name, err)
		return
	}
	service.SshUser = dict["SshUser"].(string)
	service.SshPassword = dict["SshPassword"].(string)

	log.Info("Service [%s] connect to server successfully. ExternalPort: %d, TunnelPort: %d, TunnelType: %s", service.Name, service.ExternalPort, service.TunnelPort, service.TunnelType)

	service.ControlMsgChan = make(chan int, 100)
	service.TunnelMsgChan = make(chan int, 100)

	// Start a new goroutine to create heartbeat message
	go service.heartBeatCreator()

	// Start a new goroutine to send control message
	go service.controlMsgSender()

	// Start a new goroutine to create new tunnel
	go service.tunnelCreator()

	// Listen to the control message from the server
	service.controlMsgReader()
}

func (service *Service) heartBeatCreator() {
	log.Info("Service [%s] heartbeat sender is running", service.Name)
	for {
		time.Sleep(time.Duration(service.HeartbeatTimeout/2) * time.Second)
		service.ControlMsgChan <- consts.HeartBeat
	}
}

func (service *Service) controlMsgSender() {
	log.Info("Service [%s] control message sender is running", service.Name)
	for {
		msg, ok := <-service.ControlMsgChan
		if !ok {
			log.Error("Service [%s] control message channel closed", service.Name)
			break
		}
		buf := fmt.Sprintf("%d", msg)
		bytes, err := security.AESEncryptBase64([]byte(buf), service.SecretKey)
		if err != nil {
			log.Error("Service [%s] encrypt control message failed. Error: %v", service.Name, err)
			break
		}
		err = service.ControlSocket.WriteLine(bytes)
		if err != nil {
			log.Error("Service [%s] send control message failed. Error: %v", service.Name, err)
			break
		}
	}
}

func (service *Service) tunnelCreator() {

	log.Info("Service [%s] tunnel manager is running", service.Name)
	for {
		_ = <-service.TunnelMsgChan
		var tunnel conn.Socket
		var err error
		switch strings.ToLower(service.TunnelType) {
		case "tcp", "tcp4":
			tunnel, err = conn.NewTCPSocket(ServerAddrV4, service.TunnelPort, "tcp4")
		case "tcp6":
			tunnel, err = conn.NewTCPSocket(ServerAddrV6, service.TunnelPort, "tcp6")
		case "kcp", "kcp4":
			tunnel, err = conn.NewKCPSocket(ServerAddrV4, service.TunnelPort, "udp4")
		case "kcp6":
			tunnel, err = conn.NewKCPSocket(ServerAddrV6, service.TunnelPort, "udp6")
		case "ssh":
			tunnel, err = conn.NewSSHSocket(ServerAddrV4, service.TunnelPort, service.SshPort, service.SshUser, service.SshPassword)
		case "p2p", "p2p4":
			tunnel, err = conn.NewKCPSocket(ServerAddrV4, service.TunnelPort, "udp4")
		case "p2p6":
			tunnel, err = conn.NewKCPSocket(ServerAddrV6, service.TunnelPort, "udp6")
		default:
			log.Error("Service [%s] unknown tunnel type: %s", service.Name, service.TunnelType)
		}
		if err != nil {
			log.Error("Service [%s] connect to tunnel failed. Error: %v", service.Name, err)
			continue
		}

		if strings.ToLower(service.TunnelType) != "p2p" {
			var client conn.Socket
			switch strings.ToLower(service.InternalType) {
			case "tcp", "tcp4", "tcp6":
				client, err = conn.NewTCPSocket(service.InternalAddr, service.InternalPort, "tcp")
				if err != nil {
					log.Error("Service [%s] connect to internal service failed. Error: %v", service.Name, err)
					continue
				}
			default:
				log.Error("Service [%s] unknown internal type: %s", service.Name, service.InternalType)
				continue
			}
			// create a net tunnel to process the request
			go service.tunnel(client, tunnel)
		} else {
			go service.p2pTunnel(tunnel)
		}
	}
}

func (service *Service) tunnel(client conn.Socket, tunnel conn.Socket) {
	closeFn := func(tunnel conn.Socket) {
		err := tunnel.Close()
		if err != nil {
			log.Error("Service [%s] close a tunnel failed. Error: %v", service.Name, err)
		}
	}
	defer closeFn(tunnel)
	defer closeFn(client)
	if !tunnel2.ClientTunnelSafetyCheck(tunnel, service.SecretKey) {
		log.Error("Tunnel safety check failed")
		return
	}
	if !service.TunnelEncrypt {
		tunnel2.UnsafeTunnel(client, tunnel)
		return
	} else {
		tunnel2.SafeTunnel(client, tunnel, service.SecretKey)
	}
}

func (service *Service) p2pTunnel(tunnel conn.Socket) {
	var mappingType int
	var filteringType int
	var err error
	if NATType != -1 {
		log.Info("Service [%s] Configured NAT type manually", service.Name)
		mappingType = NATType / 3
		filteringType = NATType % 3
	} else {
		log.Info("Service [%s] Start to check NAT type automatically", service.Name)
		mappingType, filteringType, err = p2p.CheckNATType("stun.miwifi.com:3478", 5)
		if err != nil {
			log.Error("Service [%s] failed to check NAT type. Error: %v", service.Name, err)
			return
		}
	}
	log.Info("Service [%s]  NAT type is set to %d(mappingType=%d, filteringType=%d)", service.Name, mappingType*3+filteringType, mappingType, filteringType)

	// Construct metadata, serialize and encrypt
	dict := make(map[string]interface{})
	dict["SecretKey"] = string(security.AesGenKey(32))
	dict["Type"] = "Client"
	dict["NATType"] = strconv.Itoa(mappingType*3 + filteringType)
	bytes, err := serialize.Serialize(&dict)
	if err != nil {
		log.Error("Service [%s] serialize metadata failed. Error: %v", service.Name, err)
		return
	}
	bytes, err = security.RSAEncryptBase64(bytes, PublicKey, NBits)
	if err != nil {
		log.Error("Service [%s] encrypt metadata failed. Error: %v", service.Name, err)
		return
	}
	err = tunnel.WriteLine(bytes)
	if err != nil {
		log.Error("Service [%s] send metadata to server failed. Error: %v", service.Name, err)
		return
	}

	bytes, err = tunnel.ReadLine()
	if err != nil {
		log.Error("Service [%s] receive response from server failed. Error: %v", service.Name, err)
		return
	}
	bytes, err = security.AESDecryptBase64(bytes, []byte(dict["SecretKey"].(string)))
	if err != nil {
		log.Error("Service [%s] decrypt response from server failed. Error: %v", service.Name, err)
		return
	}
	fmt.Println(string(bytes))

	odict := make(map[string]interface{})
	err = serialize.Deserialize(bytes, &odict)
	if err != nil {
		log.Error("Deserialize response from server failed. Error: %v", err)
		return
	}

	// UDP Hole Punching
	_ = tunnel.Close()
	laddr, err := net.ResolveUDPAddr("udp", tunnel.(*conn.KCPSocket).GetSocket().LocalAddr().String())
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
		log.Error("Service [%s] failed to get fsmFn", service.Name)
		return
	}
	fsm := fsmFn(laddr, raddr)
	if fsm == nil {
		log.Error("Service [%s] failed to get fsm", service.Name)
		return
	}
	if fsm.Run(1) != 0 {
		log.Error("Service [%s] failed to run fsm", service.Name)
		return
	}
	fmt.Printf("Service [%s] p2p tunnel is established\n", service.Name)
}

func (service *Service) controlMsgReader() {
	timer := time.AfterFunc(time.Duration(service.HeartbeatTimeout)*time.Second, func() {
		log.Error("HeartBeatTimeout ExternalPort: %d, TunnelPort: %d", service.ExternalPort, service.TunnelPort)
		err := service.ControlSocket.Close()
		if err != nil {
			log.Error("Service [%s] close control socket failed. Error: %v", service.Name, err)
			return
		}
	})
	defer timer.Stop()
	log.Info("Service [%s] control message reader is running", service.Name)
	for {
		buf, err := service.ControlSocket.ReadLine()
		if err != nil {
			log.Error("Service [%s] receive control message failed. Error: %v", service.Name, err)
			break
		}
		buf, err = security.AESDecryptBase64(buf, service.SecretKey)
		if err != nil {
			log.Error("Service [%s] decrypt control message failed. Error: %v", service.Name, err)
			break
		}
		msg, err := strconv.Atoi(string(buf))
		if err != nil {
			log.Error("Service [%s] parse control message failed. Error: %v", service.Name, err)
			break
		}
		switch msg {
		case consts.HeartBeat:
			timer.Reset(time.Duration(service.HeartbeatTimeout) * time.Second)
		case consts.CreateTunnel:
			service.TunnelMsgChan <- consts.CreateTunnel
			timer.Reset(time.Duration(service.HeartbeatTimeout) * time.Second)
		default:
			log.Warn("Service [%s] receive unknown control message: %d", service.Name, msg)
		}
	}
}

var services = make(map[string]*Service)

// RegisterService register a new service
func RegisterService(
	name string,
	internalAddr string,
	internalPort int,
	internalType string,
	externalPort int,
	externalType string,
	tunnelPort int,
	tunnelType string,
	tunnelEncrypt bool,
) {
	services[name] = &Service{
		Name:          name,
		InternalAddr:  internalAddr,
		InternalPort:  internalPort,
		InternalType:  internalType,
		ExternalPort:  externalPort,
		ExternalType:  externalType,
		TunnelPort:    tunnelPort,
		TunnelType:    tunnelType,
		TunnelEncrypt: tunnelEncrypt,
	}
}

// Run start all services
func Run() {
	log.InitLog(LogWay, LogFile, LogLevel, LogMaxDays)
	var wait sync.WaitGroup
	wait.Add(len(services))
	for _, service := range services {
		go service.run(&wait)
	}
	wait.Wait()
}
