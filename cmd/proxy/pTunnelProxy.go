package main

import (
	"errors"
	"fmt"
	"os"
	"pTunnel/proxy"
	"pTunnel/utils/version"
	"strconv"

	"github.com/docopt/docopt-go"
	"github.com/vaughan0/go-ini"
)

var usage = `pTunnelProxy is the p2p proxy for the pTunnel.
Usage:
	pTunnelProxy [options]
Options:
	-h --help                              Show help information in screen.
	--version                              Show version.
	-c --config-file=<config-file>         Specify the config file path. [default: ./conf/proxy.ini]
	--server-addr=<server-addr>            Specify the p2p server address.
	--server-port=<server-port>            Specify the p2p server port.
	-p --public-key-file=<public-key-file> Specify the public key file.
	-n --nBits-file=<nBits-file>           Specify the NBits file.
	-l --log-file=<log-level>              Specify the path to the log file.
	--log-level=<log-level>                Specify the log level. [options: debug, info, warning, error] [default: info]
	--log-max-days=<log-max-days>          Specify the log max days.
	--nat-type=<nat-type>                  Specify the NAT type. [options: 0, 1, 2, 3, 4, 5, 6, 7, 8]
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
	proxy.PublicKeyFile = args["--public-key-file"].(string)

	// NBitsFile
	if args["--nBits-file"] == nil {
		tmpStr, ok := conf.Get("common", "NBitsFile")
		if ok {
			args["--nBits-file"] = tmpStr
		} else {
			return errors.New("NBitsFile is not specified")
		}
	}
	proxy.NBitsFile = args["--nBits-file"].(string)

	// ServerAddr
	if args["--server-addr"] == nil {
		tmpStr, ok := conf.Get("common", "ServerAddr")
		if ok {
			args["--server-addr"] = tmpStr
		} else {
			return errors.New("ServerAddr is not specified")
		}
	}
	proxy.ServerAddr = args["--server-addr"].(string)

	// ServerPort
	if args["--server-port"] == nil {
		tmpStr, ok := conf.Get("common", "ServerPort")
		if ok {
			args["--server-port"] = tmpStr
		} else {
			return errors.New("ServerPort is not specified")
		}
	}
	proxy.ServerPort, err = strconv.Atoi(args["--server-port"].(string))
	if err != nil {
		return err
	}

	// P2pAddr
	if args["--p2p-addr"] == nil {
		tmpStr, ok := conf.Get("common", "P2pAddr")
		if ok {
			args["--p2p-addr"] = tmpStr
		} else {
			args["--p2p-addr"] = ""
		}
	}
	proxy.P2pAddr = args["--p2p-addr"].(string)

	// LocalType
	if args["--local-type"] == nil {
		tmpStr, ok := conf.Get("common", "LocalType")
		if ok {
			args["--local-type"] = tmpStr
		} else {
			args["--local-type"] = "tcp"
		}
	}
	proxy.LocalType = args["--local-type"].(string)

	// LocalPort
	if args["--local-port"] == nil {
		tmpStr, ok := conf.Get("common", "LocalPort")
		if ok {
			args["--local-port"] = tmpStr
		} else {
			return errors.New("LocalPort is not specified")
		}
	}
	proxy.LocalPort, err = strconv.Atoi(args["--local-port"].(string))
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
	proxy.LogFile = args["--log-file"].(string)
	if proxy.LogFile == "console" {
		proxy.LogWay = "console"
	} else {
		proxy.LogWay = "file"
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
	proxy.LogLevel = args["--log-level"].(string)

	// LogMaxDays
	if args["--log-max-days"] == nil {
		tmpStr, ok := conf.Get("common", "LogMaxDays")
		if ok {
			args["--log-max-days"] = tmpStr
		} else {
			return errors.New("LogMaxDays is not specified")
		}
	}
	proxy.LogMaxDays, err = strconv.Atoi(args["--log-max-days"].(string))
	if err != nil {
		return err
	}

	// NATType
	if args["--nat-type"] == nil {
		tmpStr, ok := conf.Get("common", "NATType")
		if ok {
			args["--nat-type"] = tmpStr
		} else {
			args["--nat-type"] = "-1"
		}
	}
	proxy.NATType, err = strconv.Atoi(args["--nat-type"].(string))
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// Parse Arguments
	args := ParseArgs()

	// Load Configurations
	err := LoadConf(args["--config-file"].(string), args)
	if err != nil {
		fmt.Printf("Error during loading configurations: %s\n", err.Error())
		return
	}

	// Initialize conf
	err = proxy.InitConf()
	if err != nil {
		fmt.Printf("Error during initializing configurations: %s\n", err.Error())
		return
	}

	// Start the proxy
	proxy.Run()
}
