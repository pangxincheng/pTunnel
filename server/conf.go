package server

import (
	"pTunnel/utils/common"
	"strconv"
)

var (
	PrivateKeyFile   string
	NBitsFile        string
	ServerType       string // tcp, tcp4, tcp6, kcp, kcp4, kcp6
	ServerPort       int
	LogFile          string
	LogWay           string
	LogLevel         string
	LogMaxDays       int
	HeartbeatTimeout int
	SshPort          int    // only for ssh tunnel
	SshUser          string // only for ssh tunnel
)

var (
	PrivateKey []byte
	NBits      int
)

func InitConf() error {
	privateKey, err := common.LoadFile(PrivateKeyFile)
	if err != nil {
		return err
	}
	PrivateKey = privateKey
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
