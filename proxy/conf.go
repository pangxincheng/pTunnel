package proxy

import (
	"pTunnel/utils/common"
	"pTunnel/utils/p2p"
	"strconv"
)

var (
	PublicKeyFile string
	NBitsFile     string
	ServerAddrV4  string
	ServerAddrV6  string
	ServerPort    int
	ServerType    string
	LogFile       string
	LogWay        string
	LogLevel      string
	LogMaxDays    int
	NatType       int
	MappingType   int
	FilteringType int
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

	if NatType != -1 {
		MappingType = NatType / 3
		FilteringType = NatType % 3
	} else {
		MappingType, FilteringType, err = p2p.CheckNATType("stun.miwifi.com:3478", 5)
		if err != nil {
			return err
		}
	}
	return nil
}
