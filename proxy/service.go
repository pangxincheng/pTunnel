package proxy

import (
	"fmt"
	"pTunnel/conn"
	"pTunnel/utils/log"
	"pTunnel/utils/p2p"
	"pTunnel/utils/security"
	"pTunnel/utils/serialize"
	"strconv"
)

type Proxy struct {
	socket *conn.KCPSocket
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
}

func Run() {
	log.InitLog(LogWay, LogFile, LogLevel, LogMaxDays)

	socket, err := conn.NewKCPSocket(ServerAddr, ServerPort)
	if err != nil {
		log.Error("Failed to create KCP Socket. Error: %v", err)
		return
	}
	proxy := &Proxy{
		socket: socket.(*conn.KCPSocket),
	}
	proxy.run()
}
