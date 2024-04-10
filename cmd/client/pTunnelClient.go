package main

import (
	"errors"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/vaughan0/go-ini"
	"os"
	"pTunnel/client"
	"pTunnel/utils/version"
	"strconv"
)

var usage = `pTunnelClient is the client application for the pTunnel.
Usage:
	pTunnelClient [options]

Options:
	-h --help                              Show help information in screen.
	--version                              Show version.
	-c --config-file=<config-file>         Specify the config file path. [default: ./conf/client.ini]
	--server-addr=<server-addr>            Specify the server address.
	--server-port=<server-port>            Specify the server port.
	-p --public-key-file=<public-key-file> Specify the public key file.
	-n --nBits-file=<nBits-file>           Specify the NBits file.
	-l --log-file=<log-level>              Specify the path to the log file.
	--log-level=<log-level>                Specify the log level. [options: debug, info, warning, error] [default: info]
	--log-max-days=<log-max-days>          Specify the log max days.
`

func ParseArgs() map[string]interface{} {
	opts, err := docopt.ParseArgs(usage, os.Args[1:], version.GetVersion())
	if err != nil {
		fmt.Printf("Error during parsing arguments: %s\n", err.Error())
		return nil
	}
	args := make(map[string]interface{})
	for k, v := range opts {
		args[k] = v
	}
	return args
}

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
			return errors.New("PublicKeyFile is not specified")
		}
	}
	client.PublicKeyFile = args["--public-key-file"].(string)

	// NBitsFile
	if args["--nBits-file"] == nil {
		tmpStr, ok := conf.Get("common", "NBitsFile")
		if ok {
			args["--nBits-file"] = tmpStr
		} else {
			return errors.New("NBitsFile is not specified")
		}
	}
	client.NBitsFile = args["--nBits-file"].(string)

	// ServerAddr
	if args["--server-addr"] == nil {
		tmpStr, ok := conf.Get("common", "ServerAddr")
		if ok {
			args["--server-addr"] = tmpStr
		} else {
			return errors.New("ServerAddr is not specified")
		}
	}
	client.ServerAddr = args["--server-addr"].(string)

	// ServerPort
	if args["--server-port"] == nil {
		tmpStr, ok := conf.Get("common", "ServerPort")
		if ok {
			args["--server-port"] = tmpStr
		} else {
			return errors.New("ServerPort is not specified")
		}
	}
	client.ServerPort, err = strconv.Atoi(args["--server-port"].(string))
	if err != nil {
		return err
	}

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

	// set services
	for k, v := range conf {
		if k != "common" {
			name := k
			internalAddr := v["InternalAddr"]
			internalPort, err := strconv.Atoi(v["InternalPort"])
			if err != nil {
				return err
			}
			internalType := v["InternalType"]
			externalPort, err := strconv.Atoi(v["ExternalPort"])
			if err != nil {
				return err
			}
			externalType := v["ExternalType"]
			tunnelType := v["TunnelType"]
			tunnelEncrypt, err := strconv.ParseBool(v["TunnelEncrypt"])
			if err != nil {
				return err
			}
			client.RegisterService(name, internalAddr, internalPort, internalType, externalPort, externalType, tunnelType, tunnelEncrypt)
		}
	}

	return nil
}

func main() {
	// Parse Arguments
	args := ParseArgs()

	// Load Configurations and register services
	err := LoadConf(args["--config-file"].(string), args)
	if err != nil {
		fmt.Printf("Error during loading configurations: %s\n", err.Error())
		return
	}

	// Initialize conf
	err = client.InitConf()
	if err != nil {
		fmt.Printf("Error during initializing configurations: %s\n", err.Error())
		return
	}

	// Start client
	client.Run()
}
