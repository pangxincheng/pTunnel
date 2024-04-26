package client

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
	"strings"
	"sync"
	"time"
)

type Service struct {
	Name             string // set mannually
	InternalAddr     string // set mannually
	InternalPort     int    // set mannually
	InternalType     string // set mannually
	ExternalPort     int    // set mannually
	ExternalType     string // set mannually
	TunnelPort       int    // set automatically/mannually
	TunnelType       string // set mannually
	TunnelEncrypt    bool   // set mannually
	HeartbeatTimeout int    // set automatically
	SshPort          int    // only for ssh tunnel, set automatically
	SshUser          string // only for ssh tunnel, set automatically
	P2PAddrV4        string // only for p2p tunnel, optional
	P2PAddrV6        string // only for p2p tunnel, optional
	P2PPort          int    // only for p2p tunnel, optional

	SecretKey []byte // set automatically

	ControlSocket  conn.Socket // set automatically
	ControlMsgChan chan int    // set automatically
	TunnelMsgChan  chan int    // set automatically
}

func (service *Service) run(wait *sync.WaitGroup) {
	defer wait.Done()
	log.Info("Service [%s] is running", service.Name)

	// Create control socket
	if service.createControlSocket() != nil {
		return
	}

	// Generate SecretKey
	service.SecretKey = security.AesGenKey(32)

	// Extract metadata
	log.Info("Service [%s] is extracting metadata", service.Name)
	if service.extractMetadata() != nil {
		return
	}
	log.Info("Service [%s] metadata extracted successfully", service.Name)

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

func (service *Service) createControlSocket() (err error) {
	// Connect to server
	service.ControlSocket, err = conn.NewSocket(
		ServerType,
		consts.Auto, consts.Auto, 0,
		ServerAddrV4, ServerAddrV6, ServerPort,
		consts.UnConf, nil,
	)
	if err != nil {
		log.Error("Service [%s] connect to server failed. Error: %v", service.Name, err)
	}
	return
}

func (service *Service) extractMetadata() (err error) {
	dict := make(map[string]interface{})
	dict["SecretKey"] = string(service.SecretKey)
	dict["ExternalPort"] = strconv.Itoa(service.ExternalPort)
	dict["ExternalType"] = service.ExternalType
	dict["TunnelPort"] = strconv.Itoa(service.TunnelPort)
	dict["TunnelType"] = service.TunnelType
	dict["TunnelEncrypt"] = service.TunnelEncrypt
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

	// Send metadata to the server
	if err = service.ControlSocket.WriteLine(bytes); err != nil {
		log.Error("Service [%s] send metadata failed. Error: %v", service.Name, err)
		return
	}

	// Receive metadata from the server
	bytes, err = service.ControlSocket.ReadLine()
	if err != nil {
		log.Error("Service [%s] receive metadata failed. Error: %v", service.Name, err)
		return
	}
	bytes, err = security.AESDecryptBase64(bytes, service.SecretKey)
	if err != nil {
		log.Error("Service [%s] decrypt metadata failed. Error: %v", service.Name, err)
		return
	}
	dict = make(map[string]interface{})
	if err = serialize.Deserialize(bytes, &dict); err != nil {
		log.Error("Service [%s] deserialize metadata failed. Error: %v", service.Name, err)
		return
	}
	var status int
	if status, err = strconv.Atoi(dict["Status"].(string)); err != nil || status != 200 {
		log.Error("Service [%s] metadata status is not 200. Error: %v", service.Name, err)
		return
	}
	service.TunnelPort, err = strconv.Atoi(dict["TunnelPort"].(string))
	if err != nil {
		log.Error("Service [%s] extract tunnel port failed. Error: %v", service.Name, err)
		return
	}
	service.HeartbeatTimeout, err = strconv.Atoi(dict["HeartbeatTimeout"].(string))
	if err != nil {
		log.Error("Service [%s] extract heartbeat timeout failed. Error: %v", service.Name, err)
		return
	}
	if service.TunnelType == "ssh" {
		service.SshPort, err = strconv.Atoi(dict["SshPort"].(string))
		if err != nil || service.SshPort == 0 {
			log.Error("Service [%s] extract ssh port failed. Error: %v", service.Name, err)
			return
		}
		service.SshUser = dict["SshUser"].(string)
	}

	log.Info("Service [%s] metadata extracted successfully", service.Name)
	return
}

func (service *Service) heartBeatCreator() {
	log.Info("Service [%s] heartbeat sender is running", service.Name)
	for {
		time.Sleep(time.Duration(service.HeartbeatTimeout/2) * time.Second)
		service.ControlMsgChan <- consts.Heartbeat
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
		socketType := service.TunnelType
		if service.TunnelType == "p2p6" {
			socketType = "kcp6"
		} else if service.TunnelType == "p2p4" {
			socketType = "kcp4"
		}
		tunnel, err := conn.NewSocket(
			socketType,
			consts.Auto, consts.Auto, 0,
			ServerAddrV4, ServerAddrV6,
			service.TunnelPort, consts.UnConf, nil,
		)
		if err != nil {
			log.Error("Service [%s] create a new tunnel failed. Error: %v", service.Name, err)
			continue
		}
		if !strings.HasPrefix(strings.ToLower(service.TunnelType), "p2p") {
			client, err := conn.NewSocket(
				service.InternalType,
				consts.Auto, consts.Auto, 0,
				service.InternalAddr, service.InternalAddr,
				service.InternalPort, consts.UnConf, nil,
			)
			if err != nil {
				tunnel.Close()
				log.Error("Service [%s] create a new client failed. Error: %v", service.Name, err)
				continue
			}
			go service.tunnel(client, tunnel, &service.SecretKey)
		} else {
			go service.p2pTunnel(tunnel)
		}
	}
}

func (service *Service) tunnel(client conn.Socket, tunnel conn.Socket, secretKey *[]byte) {
	defer tunnel.Close()
	defer client.Close()
	if !tunnel2.ClientTunnelSafetyCheck(tunnel, *secretKey) {
		log.Error("Tunnel safety check failed")
		return
	}
	if !service.TunnelEncrypt {
		tunnel2.UnsafeTunnel(client, tunnel)
		return
	} else {
		tunnel2.SafeTunnel(client, tunnel, *secretKey)
	}
}

func (service *Service) p2pTunnel(tunnel conn.Socket) {
	var RAddr *net.UDPAddr
	var LAddr *net.UDPAddr
	var FSMType string
	var SecretKey []byte

	extractMetadata := func() (err error) {
		secretKey := security.AesGenKey(32)
		dict := make(map[string]interface{})
		if service.P2PAddrV4 != "" {
			dict["Addr"] = service.P2PAddrV4
			dict["Port"] = strconv.Itoa(service.P2PPort)
			dict["Network"] = "udp4"
		} else if service.P2PAddrV6 != "" {
			dict["Addr"] = service.P2PAddrV6
			dict["Port"] = strconv.Itoa(service.P2PPort)
			dict["Network"] = "udp6"
		}
		dict["Type"] = "Client"
		dict["NATType"] = strconv.Itoa(MappingType*3 + FilteringType)
		dict["SecretKey"] = string(secretKey)
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
		if err = tunnel.WriteLine(bytes); err != nil {
			log.Error("Service [%s] send metadata failed. Error: %v", service.Name, err)
			return
		}

		bytes, err = tunnel.ReadLine()
		if err != nil {
			log.Error("Service [%s] receive metadata failed. Error: %v", service.Name, err)
			return
		}

		bytes, err = security.AESDecryptBase64(bytes, secretKey)
		if err != nil {
			log.Error("Service [%s] decrypt metadata failed. Error: %v", service.Name, err)
			return
		}

		dict = make(map[string]interface{})
		err = serialize.Deserialize(bytes, &dict)
		if err != nil {
			log.Error("Service [%s] deserialize metadata failed. Error: %v", service.Name, err)
			return
		}

		RAddr, err = net.ResolveUDPAddr(dict["RNetwork"].(string), fmt.Sprintf("%s:%s", dict["RAddr"].(string), dict["RPort"].(string)))
		if err != nil {
			log.Error("Service [%s] resolve remote address failed. Error: %v", service.Name, err)
			return
		}
		if service.P2PAddrV4 != "" {
			LAddr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", service.P2PAddrV4, strconv.Itoa(service.P2PPort)))
		} else if service.P2PAddrV6 != "" {
			LAddr, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%s", service.P2PAddrV6, strconv.Itoa(service.P2PPort)))
		} else if service.TunnelType == "p2p4" {
			LAddr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", "0.0.0.0", strconv.Itoa(service.TunnelPort)))
		} else if service.TunnelType == "p2p6" {
			LAddr, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("%s:%s", "[::]", strconv.Itoa(service.TunnelPort)))
		}
		if err != nil {
			log.Error("Resolve local address failed. Error: %v", err)
			return
		}
		FSMType = dict["FSMType"].(string)
		SecretKey = []byte(dict["SecretKey"].(string))
		return
	}

	closeTunnelSocket := func() {
		if tunnel != nil {
			tunnel.Close()
		}
		tunnel = nil
	}

	udpHolePunching := func() (err error) {
		fsmFn := p2p.GetFSM(FSMType)
		if fsmFn == nil {
			log.Error("Service [%s] get FSM failed", service.Name)
			err = errors.New("unsupported FSM type")
			return
		}
		fsm := fsmFn(LAddr, RAddr)
		if fsm == nil {
			log.Error("Service [%s] create FSM failed", service.Name)
			err = errors.New("create FSM failed")
			return
		}
		if fsm.Run(1) != 0 {
			log.Error("Service [%s] run FSM failed", service.Name)
			err = errors.New("run FSM failed")
			return
		}
		log.Info("Service [%s] UDP hole punching success", service.Name)
		tunnel = fsm.GetKCPSocket()
		return
	}

	defer closeTunnelSocket()

	if extractMetadata() != nil {
		return
	}

	closeTunnelSocket()

	if udpHolePunching() != nil {
		return
	}

	client, err := conn.NewSocket(
		service.InternalType,
		consts.Auto, consts.Auto, 0,
		service.InternalAddr, service.InternalAddr,
		service.InternalPort, consts.UnConf, nil,
	)
	if err != nil {
		log.Error("Service [%s] create a new client failed. Error: %v", service.Name, err)
		return
	}
	service.tunnel(client, tunnel, &SecretKey)
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
		case consts.Heartbeat:
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
	p2pAddrV4 string,
	p2pAddrV6 string,
	p2pPort int,
) {
	if _, ok := services[name]; ok {
		panic("service already exists")
	}
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
		P2PAddrV4:     p2pAddrV4,
		P2PAddrV6:     p2pAddrV6,
		P2PPort:       p2pPort,
	}
}

func Run() {
	log.InitLog(LogFile, LogWay, LogLevel, LogMaxDays)
	var wait sync.WaitGroup
	wait.Add(len(services))
	for _, service := range services {
		go service.run(&wait)
	}
	wait.Wait()
}
