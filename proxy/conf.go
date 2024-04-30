package proxy

import (
	"pTunnel/utils/common"
	"strconv"
)

var (
	PublicKeyFile string
	NBitsFile     string
	ServerAddr    string
	ServerPort    int
	P2pAddr       string
	LocalType     string
	LocalPort     int
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
