package client

import (
	"pTunnel/utils/common"
	"strconv"
)

// Configurations for the client
var (
	PublicKeyFile string
	NBitsFile     string
	ServerAddrV4  string
	ServerAddrV6  string
	ServerType    string
	ServerPort    int
	LogFile       string
	LogWay        string
	LogLevel      string
	LogMaxDays    int
	NATType       int = -1
)

var (
	PublicKey []byte
	NBits     int
)

// InitConf initializes the configurations
func InitConf() error {
	publicKey, err := common.LoadFile(PublicKeyFile)
	if err != nil {
		return err
	}
	PublicKey = publicKey
	nBits, err := common.LoadFile(NBitsFile)
	if err != nil {
		return err
	}
	NBits, err = strconv.Atoi(string(nBits))
	if err != nil {
		return err
	}
	return nil
}
