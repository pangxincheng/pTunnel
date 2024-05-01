package main

import (
	"errors"
	"fmt"
	"pTunnel/client"
	"pTunnel/utils/common"
	"strconv"
	"strings"

	"github.com/vaughan0/go-ini"
)

var usage = `pTunnelClient is the client application for the pTunnel.
Usage:
	  pTunnelClient [options]
	
Options:
	-h --help                              Show help information in screen.
	-v --version                           Show version.
	--config-file=<config-file>            Specify the config file path. [default: ./conf/client.ini]
	--public-key-file=<public-key-file>    Specify the public key file.
	--nBits-file=<nBits-file>              Specify the NBits file.
	--server-addr-v4=<server-addr-v4>      Specify the server ipv4 address.
	--server-addr-v6=<server-addr-v6>      Specify the server ipv6 address.
	--server-port=<server-port>            Specify the server port.
	--server-type=<server-type>            Specify the server type. [options: tcp, tcp4, tcp6, kcp, kcp4, kcp6]
	--log-file=<log-level>                 Specify the path to the log file.
	--log-level=<log-level>                Specify the log level. [options: debug, info, warning, error] [default: info]
	--log-max-days=<log-max-days>          Specify the log max days.
	--nat-type=<nat-type>                  Specify the NAT type. [options: 0, 1, 2, 3, 4, 5, 6, 7, 8]
	--ssh-private-key-file=<ssh-private-key-file> Specify the ssh private key file.
`

func LoadConf(confFile string, args map[string]interface{}) error {
	conf, err := ini.LoadFile(confFile)
	if err != nil {
		return err
	}

	// PublicKeyFile
	if args["--public-key-file"] == nil {
		tmpStr, ok := conf.Get("common", "PublicKeyFile")
		if ok {
			args["--public-key-file"] = tmpStr
		} else {
			return fmt.Errorf("PublicKeyFile is not specified")
		}
	}
	client.PublicKeyFile = args["--public-key-file"].(string)

	// NBitsFile
	if args["--nBits-file"] == nil {
		tmpStr, ok := conf.Get("common", "NBitsFile")
		if ok {
			args["--nBits-file"] = tmpStr
		} else {
			return fmt.Errorf("NBitsFile is not specified")
		}
	}
	client.NBitsFile = args["--nBits-file"].(string)

	// ServerAddrV4
	if args["--server-addr-v4"] == nil {
		tmpStr, ok := conf.Get("common", "ServerAddrV4")
		if ok {
			args["--server-addr-v4"] = tmpStr
		} else {
			fmt.Println("ServerAddrV4 is not specified, set to \"\"")
			args["--server-addr-v4"] = ""
		}
	}
	client.ServerAddrV4 = args["--server-addr-v4"].(string)

	// ServerAddrV6
	if args["--server-addr-v6"] == nil {
		tmpStr, ok := conf.Get("common", "ServerAddrV6")
		if ok {
			args["--server-addr-v6"] = tmpStr
		} else {
			fmt.Println("ServerAddrV6 is not specified, set to \"\"")
			args["--server-addr-v6"] = ""
		}
	}
	client.ServerAddrV6 = args["--server-addr-v6"].(string)

	// ServerPort
	if args["--server-port"] == nil {
		tmpStr, ok := conf.Get("common", "ServerPort")
		if ok {
			args["--server-port"] = tmpStr
		} else {
			return fmt.Errorf("ServerPort is not specified")
		}
	}
	client.ServerPort, err = strconv.Atoi(args["--server-port"].(string))
	if err != nil {
		return err
	}

	// ServerType
	if args["--server-type"] == nil {
		tmpStr, ok := conf.Get("common", "ServerType")
		if ok {
			args["--server-type"] = tmpStr
		} else {
			return errors.New("ServerType is not specified")
		}
	}
	client.ServerType = args["--server-type"].(string)

	// LogFile
	if args["--log-file"] == nil {
		tmpStr, ok := conf.Get("common", "LogFile")
		if ok {
			args["--log-file"] = tmpStr
		} else {
			return errors.New("LogFile is not specified")
		}
	}
	client.LogFile = args["--log-file"].(string)
	if client.LogFile == "console" {
		client.LogWay = "console"
	} else {
		client.LogWay = "file"
	}

	// LogLevel
	if args["--log-level"] == nil {
		tmpStr, ok := conf.Get("common", "LogLevel")
		if ok {
			args["--log-level"] = tmpStr
		} else {
			return errors.New("LogLevel is not specified")
		}
	}
	client.LogLevel = args["--log-level"].(string)

	// LogMaxDays
	if args["--log-max-days"] == nil {
		tmpStr, ok := conf.Get("common", "LogMaxDays")
		if ok {
			args["--log-max-days"] = tmpStr
		} else {
			return errors.New("LogMaxDays is not specified")
		}
	}
	client.LogMaxDays, err = strconv.Atoi(args["--log-max-days"].(string))
	if err != nil {
		return err
	}

	// NatType
	if args["--nat-type"] == nil {
		tmpStr, ok := conf.Get("common", "NatType")
		if ok {
			args["--nat-type"] = tmpStr
		} else {
			fmt.Println("NatType is not specified, set to -1")
			args["--nat-type"] = "-1"
		}
	}
	client.NatType, err = strconv.Atoi(args["--nat-type"].(string))
	if err != nil {
		return err
	}

	// SSHPrivateKeyFile
	if args["--ssh-private-key-file"] == nil {
		tmpStr, ok := conf.Get("common", "SSHPrivateKeyFile")
		if ok {
			args["--ssh-private-key-file"] = tmpStr
		} else {
			args["--ssh-private-key-file"] = ""
		}
	}
	client.SSHPrivateKeyFile = args["--ssh-private-key-file"].(string)

	for k, v := range conf {
		if k != "common" {
			name := k
			internalAddr := v["InternalAddr"]
			internalPort, err := strconv.Atoi(v["InternalPort"])
			if err != nil {
				return err
			}
			internalType := v["InternalType"]
			tunnelPort := 0
			if _, ok := v["TunnelPort"]; ok {
				tunnelPort, err = strconv.Atoi(v["TunnelPort"])
				if err != nil {
					return err
				}
			}
			tunnelType := v["TunnelType"]
			tunnelEncrypt, err := strconv.ParseBool(v["TunnelEncrypt"])
			if err != nil {
				return err
			}
			externalPort := tunnelPort
			externalType := tunnelType
			p2pAddrV4 := ""
			p2pAddrV6 := ""
			if !strings.HasPrefix(strings.ToLower(tunnelType), "p2p") {
				externalPort, err = strconv.Atoi(v["ExternalPort"])
				if err != nil {
					return err
				}
				externalType = v["ExternalType"]
			} else {
				if _, ok := v["P2PAddrV4"]; ok {
					p2pAddrV4 = v["P2PAddrV4"]
				}
				if _, ok := v["P2PAddrV6"]; ok {
					p2pAddrV6 = v["P2PAddrV6"]
				}
			}
			client.RegisterService(
				name,
				internalAddr, internalPort, internalType,
				externalPort, externalType,
				tunnelPort, tunnelType, tunnelEncrypt,
				p2pAddrV4, p2pAddrV6,
			)
		}
	}
	return nil
}

func main() {
	// Parse arguments
	args := common.ParseArgs(&usage)
	if args == nil {
		return
	}

	// Load configuration
	err := LoadConf(args["--config-file"].(string), args)
	if err != nil {
		fmt.Printf("Error during loading configurations: %v\n", err)
		return
	}

	// Initialize conf
	err = client.InitConf()
	if err != nil {
		fmt.Printf("Error during initializing configurations: %v\n", err)
		return
	}

	// Start the client
	client.Run()
}
