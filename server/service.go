package server

import (
	"errors"
	"fmt"
	"pTunnel/conn"
	tunnel2 "pTunnel/tunnel"
	"pTunnel/utils/consts"
	"pTunnel/utils/log"
	"pTunnel/utils/security"
	"pTunnel/utils/serialize"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	ControlSocket    conn.Socket
	SecretKey        []byte
	ExternalPort     int
	ExternalType     string
	ExternalListener conn.Listener
	TunnelEncrypt    bool
	TunnelPort       int
	TunnelType       string
	TunnelListener   conn.Listener
	SshPort          int    // only for ssh tunnel
	SshUser          string // only for ssh tunnel
	SshPassword      string // only for ssh tunnel

	ControlMsgChan chan int
	WorkerChan     chan conn.Socket
	RequestChan    chan conn.Socket
}

func (service *Service) run() {
	defer func(ControlSocket conn.Socket) {
		_ = ControlSocket.Close()
	}(service.ControlSocket)

	// Read the metadata from the client, decrypt and deserialize
	bytes, err := service.ControlSocket.ReadLine()
	if err != nil {
		log.Error("Failed to read metadata from the client. Error: %v", err)
		return
	}
	bytes, err = security.RSADecryptBase64(bytes, PrivateKey, NBits)
	if err != nil {
		log.Error("Failed to decrypt metadata from the client. Error: %v", err)
		return
	}
	dict := make(map[string]interface{})
	err = serialize.Deserialize(bytes, &dict)
	if err != nil {
		log.Error("Failed to deserialize metadata from the client. Error: %v", err)
		return
	}

	// Extract the metadata
	service.SecretKey = []byte(dict["SecretKey"].(string))
	service.ExternalPort, err = strconv.Atoi(dict["ExternalPort"].(string))
	if err != nil {
		log.Error("Failed to convert ExternalPort to integer. Error: %v", err)
		return
	}
	service.ExternalType = dict["ExternalType"].(string)
	service.TunnelEncrypt = dict["TunnelEncrypt"].(bool)
	service.TunnelType = dict["TunnelType"].(string)
	if dict["TunnelPort"] != nil {
		service.TunnelPort, err = strconv.Atoi(dict["TunnelPort"].(string))
		if err != nil {
			service.TunnelPort = 0
		}
	} else {
		service.TunnelPort = 0
	}
	if strings.ToLower(service.TunnelType) == "ssh" {
		if SshPort == 0 {
			log.Error("SshPort is not set")
			return
		}
		service.SshPort = SshPort
		service.SshUser = SshUser
		service.SshPassword = SshPassword
	}

	// Create a new external listener
	err = service.createExternalListener()
	if err != nil {
		log.Error("Failed to create external listener. Error: %v", err)
		return
	}

	// Create a new tunnel listener
	err = service.createTunnelListener()
	if err != nil {
		log.Error("Failed to create tunnel listener. Error: %v", err)
		return
	}
	_, service.TunnelPort = service.TunnelListener.Address()

	// Construct response, serialize and encrypt
	dict = make(map[string]interface{})
	dict["Status"] = strconv.Itoa(200)
	dict["TunnelPort"] = strconv.Itoa(service.TunnelPort)
	dict["SshPort"] = strconv.Itoa(service.SshPort)
	dict["SshUser"] = service.SshUser
	dict["SshPassword"] = service.SshPassword
	dict["HeartbeatTimeout"] = strconv.Itoa(HeartbeatTimeout)
	bytes, err = serialize.Serialize(&dict)
	if err != nil {
		log.Error("Failed to serialize response. Error: %v", err)
		return
	}
	bytes, err = security.AESEncryptBase64(bytes, service.SecretKey)
	if err != nil {
		log.Error("Failed to encrypt response. Error: %v", err)
		fmt.Println(service.SecretKey)
		return
	}

	// Send response to the client
	err = service.ControlSocket.WriteLine(bytes)
	if err != nil {
		log.Error("Failed to send response to the client. Error: %v", err)
		return
	}

	service.ControlMsgChan = make(chan int, 100)
	service.WorkerChan = make(chan conn.Socket, 100)
	service.RequestChan = make(chan conn.Socket, 100)

	// Start a new goroutine to listen to the control message from the client
	go service.controlMsgReader()

	// Start a new goroutine to send control message to the client
	go service.controlMsgSender()

	// Start a new goroutine to:
	// 1. accept socket from the tunnel
	// 2. add it to WorkerChan
	go service.tunnelListen()

	// Start a new goroutine to process requests
	// 1. get a worker from WorkerChan
	// 2. get a request from RequestChan
	// 3. start a new goroutine to forward data between worker and request
	go service.requestProcessor()

	// 1. Listen and accept new connections from the ExternalListener
	// 2. add it to RequestChan
	// 3. add a CreateTunnel signal to ControlMsgChan
	service.serverListener()

}

func (service *Service) createExternalListener() (err error) {
	if strings.ToLower(service.ExternalType) == "tcp" {
		service.ExternalListener, err = conn.NewTCPListener("0.0.0.0", service.ExternalPort)
	} else if strings.ToLower(service.TunnelType) == "udp" {
		err = errors.New("Unsupported external type: " + service.ExternalType)
		//service.ExternalListener, err = conn.NewUDPListener("0.0.0.0", service.ExternalPort)
	} else {
		err = errors.New("Unsupported external type: " + service.ExternalType)
	}
	return
}

func (service *Service) createTunnelListener() (err error) {
	if strings.ToLower(service.TunnelType) == "tcp" {
		service.TunnelListener, err = conn.NewTCPListener("0.0.0.0", service.TunnelPort)
	} else if strings.ToLower(service.TunnelType) == "kcp" {
		service.TunnelListener, err = conn.NewKCPListener("0.0.0.0", service.TunnelPort)
	} else if strings.ToLower(service.TunnelType) == "udp" {
		err = errors.New("Unsupported tunnel type: " + service.TunnelType)
		//service.TunnelListener, err = conn.NewUDPListener("0.0.0.0", 0)
	} else if strings.ToLower(service.TunnelType) == "ssh" {
		// Actually, the service.TunnelListener is a TCPListener.
		service.TunnelListener, err = conn.NewSSHListener("0.0.0.0", service.TunnelPort)
	} else {
		err = errors.New("Unsupported tunnel type: " + service.TunnelType)
	}
	return
}

func (service *Service) controlMsgReader() {
	log.Info(
		"Control message reader(EP: %d, ET: %s, TP: %d, TT: %s) is running",
		service.ExternalPort, service.ExternalType,
		service.TunnelPort, service.TunnelType,
	)
	timer := time.AfterFunc(time.Duration(HeartbeatTimeout)*time.Second, func() {
		log.Error("HeartBeatTimeout ExternalPort: %d, TunnelPort: %d", service.ExternalPort, service.TunnelPort)
		_ = service.ControlSocket.Close()
		_ = service.ExternalListener.Close()
		_ = service.TunnelListener.Close()
	})
	defer timer.Stop()
	defer service.ControlSocket.Close()
	defer service.ExternalListener.Close()
	defer service.TunnelListener.Close()
	for {
		bytes, err := service.ControlSocket.ReadLine()
		if err != nil {
			log.Error("Failed to read control message from the client. Error: %v", err)
			return
		}
		bytes, err = security.AESDecryptBase64(bytes, service.SecretKey)
		if err != nil {
			log.Error("Failed to decrypt control message from the client. Error: %v", err)
			return
		}
		msg, err := strconv.Atoi(string(bytes))
		if err != nil {
			log.Error("Failed to parse control message from the client. Error: %v", err)
			return
		}
		switch msg {
		case consts.HeartBeat:
			service.ControlMsgChan <- consts.HeartBeat
			timer.Reset(time.Duration(HeartbeatTimeout) * time.Second)
		default:
			log.Warn("Unsupported msg: %d", msg)
		}
	}
}

func (service *Service) controlMsgSender() {
	log.Info(
		"Control message sender(EP: %d, ET: %s, TP: %d, TT: %s) is running",
		service.ExternalPort, service.ExternalType,
		service.TunnelPort, service.TunnelType,
	)
	for {
		msg, ok := <-service.ControlMsgChan
		if !ok {
			log.Error("Control message channel is closed")
			break
		}
		bytes := []byte(strconv.Itoa(msg))
		bytes, err := security.AESEncryptBase64(bytes, service.SecretKey)
		if err != nil {
			log.Error("Failed to encrypt control message. Error: %v", err)
			break
		}
		err = service.ControlSocket.WriteLine(bytes)
		if err != nil {
			log.Error("Failed to send control message. Error: %v", err)
			break
		}
	}
}

func (service *Service) tunnelListen() {
	log.Info(
		"Tunnel listener(EP: %d, ET: %s, TP: %d, TT: %s) is running",
		service.ExternalPort, service.ExternalType,
		service.TunnelPort, service.TunnelType,
	)
	for {
		accept, err := service.TunnelListener.Accept()
		if err != nil {
			log.Error("Failed to accept connection from the tunnel. Error: %v", err)
			break
		}
		service.WorkerChan <- accept
	}
}

func (service *Service) requestProcessor() {

	for {
		request, ok := <-service.RequestChan
		if !ok {
			log.Error("Request channel is closed")
			break
		}
		worker, ok := <-service.WorkerChan
		if !ok {
			log.Error("Worker channel is closed")
			break
		}
		go service.tunnel(request, worker)
	}
}

func (service *Service) tunnel(client conn.Socket, tunnel conn.Socket) {
	closeFn := func(tunnel conn.Socket) {
		err := tunnel.Close()
		if err != nil {
			log.Error("Service close a tunnel failed. Error: %v", err)
		}
	}
	defer closeFn(tunnel)
	defer closeFn(client)
	if !tunnel2.ServerTunnelSafetyCheck(tunnel, service.SecretKey) {
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

func (service *Service) serverListener() {
	log.Info(
		"Server listener(EP: %d, ET: %s, TP: %d, TT: %s) is running",
		service.ExternalPort, service.ExternalType,
		service.TunnelPort, service.TunnelType,
	)
	for {
		accept, err := service.ExternalListener.Accept()
		if err != nil {
			log.Error("Failed to accept connection from the client. Error: %v", err)
			break
		}
		service.RequestChan <- accept
		service.ControlMsgChan <- consts.CreateTunnel
	}
}

func Run() {
	log.InitLog(LogWay, LogFile, LogLevel, LogMaxDays)

	listener, err := conn.NewTCPListener("0.0.0.0", ServerPort)
	if err != nil {
		log.Error("Failed to create TCP listener. Error: %v", err)
		return
	}

	for {
		accept, err := listener.Accept()
		if err != nil {
			log.Error("Failed to accept connection. Error: %v", err)
			continue
		}
		service := &Service{
			ControlSocket: accept,
		}
		go service.run()
	}
}
