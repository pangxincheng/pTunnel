package server

import (
	"errors"
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
	TunnelType       string
	TunnelPort       int
	TunnelListener   conn.Listener

	SshPort int    // only for ssh tunnel
	SshUser string // only for ssh tunnel

	ControlMsgChan chan int
	WorkerChan     chan *map[string]interface{}
	RequestChan    chan *map[string]interface{}
}

func (service *Service) run() {
	log.Info("Start a new go routine to handle the connection from %s to %s", service.ControlSocket.RemoteAddr(), service.ControlSocket.LocalAddr())
	defer func(ControlSocket conn.Socket) {
		_ = ControlSocket.Close()
	}(service.ControlSocket)

	// Extract metadata
	if service.extractMetadata() != nil {
		return
	}

	// Create a new external listener
	if service.createExternalListener() != nil {
		return
	}

	// Create a new tunnel listener
	if service.createTunnelListener() != nil {
		return
	}

	// Send the metadata to the client
	if service.sendMetadataToClient() != nil {
		return
	}

	service.ControlMsgChan = make(chan int, 100)
	service.WorkerChan = make(chan *map[string]interface{}, 100)
	service.RequestChan = make(chan *map[string]interface{}, 100)

	// Start a new goroutine to listen to the control message from the client
	go service.controlMsgReader()

	// Start a new goroutine to send control message to the client
	go service.controlMsgSender()

	switch strings.ToLower(service.ExternalType) {
	case "p2p", "p2p4", "p2p6":
		go service.p2pTunnelListener()
		service.p2pRequestProcessor()
	default:
		// Start a new goroutine to:
		// 1. accept socket from the tunnel
		// 2. add it to WorkerChan
		go service.tunnelListener()

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
}

func (service *Service) extractMetadata() (err error) {
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
	service.SecretKey = []byte(dict["SecretKey"].(string))
	service.ExternalPort, err = strconv.Atoi(dict["ExternalPort"].(string))
	if err != nil {
		log.Error("Failed to convert ExternalPort to int. Error: %v", err)
		return
	}
	service.ExternalType = dict["ExternalType"].(string)
	service.TunnelEncrypt = dict["TunnelEncrypt"].(bool)
	service.TunnelType = dict["TunnelType"].(string)
	TunnelPort, ok := dict["TunnelPort"].(string)
	if !ok {
		TunnelPort = "0"
	}
	service.TunnelPort, err = strconv.Atoi(TunnelPort)
	if err != nil {
		log.Error("Failed to convert TunnelPort to int. Error: %v", err)
		return
	}
	if strings.HasPrefix(strings.ToLower(service.TunnelType), "ssh") {
		if SshPort == 0 {
			log.Error("SshPort is not set")
			err = errors.New("SshPort is not set")
			return
		}
		service.SshPort = SshPort
		service.SshUser = SshUser
	}
	return
}

func (service *Service) createExternalListener() (err error) {
	switch strings.ToLower(service.ExternalType) {
	case "tcp4", "tcp6":
		service.ExternalListener, err = conn.NewListener(service.ExternalType, consts.Auto, service.ExternalPort)
	case "p2p4":
		service.ExternalListener, err = conn.NewListener("kcp4", consts.Auto, service.ExternalPort)
	case "p2p6":
		service.ExternalListener, err = conn.NewListener("kcp6", consts.Auto, service.ExternalPort)
	default:
		err = errors.New("unsupported ExternalType")
	}
	if err != nil {
		log.Error("Failed to create external listener. Error: %v", err)
	}
	return
}

func (service *Service) createTunnelListener() (err error) {
	switch strings.ToLower(service.TunnelType) {
	case "tcp", "tcp4", "tcp6", "kcp", "kcp4", "kcp6", "ssh", "ssh4", "ssh6":
		service.TunnelListener, err = conn.NewListener(service.TunnelType, "[auto]", service.TunnelPort)
	case "p2p", "p2p4", "p2p6":
		service.TunnelListener = service.ExternalListener
	default:
		err = errors.New("unsupported TunnelType")
	}
	if err != nil {
		log.Error("Failed to create tunnel listener. Error: %v", err)
	} else {
		address := strings.Split(service.TunnelListener.Address().String(), ":")
		service.TunnelPort, err = strconv.Atoi(address[len(address)-1])
		if err != nil {
			log.Error("Failed to convert TunnelPort to int. Error: %v", err)
		}
	}
	return
}

func (service *Service) sendMetadataToClient() (err error) {
	dict := make(map[string]interface{})
	dict["Status"] = strconv.Itoa(200)
	dict["TunnelPort"] = strconv.Itoa(service.TunnelPort)
	dict["SshPort"] = strconv.Itoa(service.SshPort)
	dict["SshUser"] = service.SshUser
	dict["HeartbeatTimeout"] = strconv.Itoa(HeartbeatTimeout)
	bytes, err := serialize.Serialize(&dict)
	if err != nil {
		log.Error("Failed to serialize metadata. Error: %v", err)
		return
	}
	bytes, err = security.AESEncryptBase64(bytes, service.SecretKey)
	if err != nil {
		log.Error("Failed to encrypt metadata. Error: %v", err)
		return
	}
	err = service.ControlSocket.WriteLine(bytes)
	if err != nil {
		log.Error("Failed to send metadata to the client. Error: %v", err)
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
		case consts.Heartbeat:
			service.ControlMsgChan <- consts.Heartbeat
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

func (service *Service) tunnelListener() {
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
		service.WorkerChan <- &map[string]interface{}{
			"Socket": accept,
		}
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
		go service.tunnel((*request)["Socket"].(conn.Socket), (*worker)["Socket"].(conn.Socket))
	}
}

func (service *Service) tunnel(client conn.Socket, tunnel conn.Socket) {
	defer tunnel.Close()
	defer client.Close()
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
		service.RequestChan <- &map[string]interface{}{
			"Socket": accept,
		}
		service.ControlMsgChan <- consts.CreateTunnel
	}
}

func (service *Service) p2pTunnelListener() {
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
		// check whether the accept is a client / a proxy
		go func(accept conn.Socket) {
			bytes, err := accept.ReadLine()
			if err != nil {
				log.Error("Failed to read the first line from the tunnel. Error: %v", err)
				return
			}
			bytes, err = security.RSADecryptBase64(bytes, PrivateKey, NBits)
			if err != nil {
				log.Error("Failed to decrypt the first line from the tunnel. Error: %v", err)
				return
			}
			dict := make(map[string]interface{})
			err = serialize.Deserialize(bytes, &dict)
			if err != nil {
				log.Error("Failed to deserialize metadata from the client. Error: %v", err)
				return
			}
			if dict["Type"].(string) == "Proxy" {
				// add it to RequestChan
				service.RequestChan <- &map[string]interface{}{
					"Socket":   accept,
					"Metadata": dict,
				}
				// add a CreateTunnel signal to ControlMsgChan to create a new tunnel
				service.ControlMsgChan <- consts.CreateTunnel
			} else {
				// add it to WorkerChan
				service.WorkerChan <- &map[string]interface{}{
					"Socket":   accept,
					"Metadata": dict,
				}
			}
		}(accept)
	}
}

func (service *Service) p2pRequestProcessor() {
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
		reqSocket := (*request)["Socket"].(conn.Socket)
		reqMetadata := (*request)["Metadata"].(map[string]interface{})
		workerSocket := (*worker)["Socket"].(conn.Socket)
		workerMetadata := (*worker)["Metadata"].(map[string]interface{})
		go service.p2pTunnel(reqSocket, workerSocket, reqMetadata, workerMetadata)
	}
}

var natType2FsmForProxy = [9][9]string{
	{"Fn10", "Fn10", "Fn10", "Fn10", "Fn10", "Fn10", "Fn10", "Fn10", "Fn10"},
	{"Fn11", "Fn20", "Fn20", "Fn30", "Fn30", "Fn30", "Fn30", "Fn30", "Fn30"},
	{"Fn11", "Fn20", "Fn20", "Fn30", "Fn30", "Fn30", "Fn30", "Fn30", "Fn30"},
	{"Fn11", "Fn31", "Fn31", "", "", "", "", "", ""},
	{"Fn11", "Fn31", "Fn31", "", "", "", "", "", ""},
	{"Fn11", "Fn31", "Fn31", "", "", "", "", "", ""},
	{"Fn11", "Fn31", "Fn31", "", "", "", "", "", ""},
	{"Fn11", "Fn31", "Fn31", "", "", "", "", "", ""},
	{"Fn11", "Fn31", "Fn31", "", "", "", "", "", ""},
}

var natType2FsmForTunnel = [9][9]string{
	{"Fn11", "Fn11", "Fn11", "Fn11", "Fn11", "Fn11", "Fn11", "Fn11", "Fn11"},
	{"Fn10", "Fn21", "Fn21", "Fn31", "Fn31", "Fn31", "Fn31", "Fn31", "Fn31"},
	{"Fn10", "Fn21", "Fn21", "Fn31", "Fn31", "Fn31", "Fn31", "Fn31", "Fn31"},
	{"Fn10", "Fn30", "Fn30", "", "", "", "", "", ""},
	{"Fn10", "Fn30", "Fn30", "", "", "", "", "", ""},
	{"Fn10", "Fn30", "Fn30", "", "", "", "", "", ""},
	{"Fn10", "Fn30", "Fn30", "", "", "", "", "", ""},
	{"Fn10", "Fn30", "Fn30", "", "", "", "", "", ""},
	{"Fn10", "Fn30", "Fn30", "", "", "", "", "", ""},
}

func (service *Service) p2pTunnel(proxy conn.Socket, tunnel conn.Socket, proxyMetadata map[string]interface{}, tunnelMetadata map[string]interface{}) {
	pNatType, err := strconv.Atoi(proxyMetadata["NATType"].(string))
	if err != nil {
		log.Error("Failed to convert client NAT type to integer. Error: %v", err)
		return
	}
	tNatType, err := strconv.Atoi(tunnelMetadata["NATType"].(string))
	if err != nil {
		log.Error("Failed to convert tunnel NAT type to integer. Error: %v", err)
		return
	}

	secretKey := security.AesGenKey(32)

	// send to the tunnel
	metadata := make(map[string]interface{})
	if addr, ok := proxyMetadata["Addr"]; ok {
		log.Info("The proxy has configured addr manually: %s", addr)
		metadata["RAddr"] = addr
		metadata["RPort"] = proxyMetadata["Port"]
		metadata["RNetwork"] = proxyMetadata["Network"]
	} else {
		log.Info("The proxy has not configured addr manually, use the remote addr of the proxy.")
		raddress := strings.Split(proxy.RemoteAddr().String(), ":")
		metadata["RPort"] = raddress[len(raddress)-1]
		metadata["RAddr"] = strings.Join(raddress[:len(raddress)-1], ":")
		metadata["RNetwork"] = "udp4"
		if strings.Contains(metadata["RAddr"].(string), ":") {
			metadata["RNetwork"] = "udp6"
		}
	}
	metadata["FSMType"] = natType2FsmForTunnel[pNatType][tNatType]
	metadata["SecretKey"] = secretKey
	metadata["TunnelEncrypt"] = service.TunnelEncrypt
	bytes, err := serialize.Serialize(&metadata)
	if err != nil {
		log.Error("Failed to serialize client metadata. Error: %v", err)
		return
	}
	bytes, err = security.AESEncryptBase64(bytes, []byte(tunnelMetadata["SecretKey"].(string)))
	if err != nil {
		log.Error("Failed to encrypt client metadata. Error: %v", err)
		return
	}
	err = tunnel.WriteLine(bytes)
	if err != nil {
		log.Error("Failed to send client metadata to the tunnel. Error: %v", err)
		return
	}

	// send to the proxy
	metadata = make(map[string]interface{})
	if addr, ok := tunnelMetadata["Addr"]; ok {
		log.Info("The tunnel has configured addr manually: %s", addr)
		metadata["RAddr"] = addr
		metadata["RPort"] = tunnelMetadata["Port"]
		metadata["RNetwork"] = tunnelMetadata["Network"]
	} else {
		log.Info("The tunnel has not configured addr manually, use the remote addr of the tunnel.")
		raddress := strings.Split(tunnel.RemoteAddr().String(), ":")
		metadata["RPort"] = raddress[len(raddress)-1]
		metadata["RAddr"] = strings.Join(raddress[:len(raddress)-1], ":")
		metadata["RNetwork"] = "udp4"
		if strings.Contains(metadata["RAddr"].(string), ":") {
			metadata["RNetwork"] = "udp6"
		}
	}
	metadata["FSMType"] = natType2FsmForProxy[pNatType][tNatType]
	metadata["SecretKey"] = secretKey
	metadata["TunnelEncrypt"] = service.TunnelEncrypt
	bytes, err = serialize.Serialize(&metadata)
	if err != nil {
		log.Error("Failed to serialize tunnel metadata. Error: %v", err)
		return
	}
	bytes, err = security.AESEncryptBase64(bytes, []byte(proxyMetadata["SecretKey"].(string)))
	if err != nil {
		log.Error("Failed to encrypt tunnel metadata. Error: %v", err)
		return
	}
	err = proxy.WriteLine(bytes)
	if err != nil {
		log.Error("Failed to send tunnel metadata to the client. Error: %v", err)
		return
	}
	time.Sleep(1 * time.Second)
}

func Run() {
	log.InitLog(LogWay, LogFile, LogLevel, LogMaxDays)
	listener, err := conn.NewListener(ServerType, consts.Auto, ServerPort)
	if err != nil {
		log.Error("Failed to create listener: %v", err)
		return
	}
	log.Info("Server started at %s", listener.Address().String())
	for {
		accept, err := listener.Accept()
		if err != nil {
			log.Error("Failed to accept connection: %v", err)
			continue
		}
		service := &Service{
			ControlSocket: accept,
		}
		go service.run()
	}
}
