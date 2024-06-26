package main

import (
	"errors"
	"fmt"
	"pTunnel/server"
	"pTunnel/utils/common"
	"strconv"

	"github.com/vaughan0/go-ini"
)

var usage = `pTunnelServer is the server application for the pTunnel.
Usage:
	pTunnelServer [options]

Options:
	-h --help                                Show help information in screen.
	-v --version                             Show version.
	--config-file=<config-file>              Specify the config file path. [default: ./conf/server.ini]
	--private-key-file=<private-key-file>    Specify the private key file.
	--nBits-file=<nBits-file>                Specify the NBits file.
	--server-type=<server-type>              Specify the server type. [options: tcp, tcp4, tcp6]
	--server-port=<server-port>              Specify the server port.
	--log-file=<log-level>                   Specify the path to the log file.
	--log-level=<log-level>                  Specify the log level. [options: debug, info, warning, error]
	--log-max-days=<log-max-days>            Specify the log max days.
	--heartbeat-timeout=<heartbeat-timeout>  Specify the heartbeat timeout. 
	--ssh-port=<ssh-port>                    Specify the ssh port.
	--ssh-user=<ssh-user>                    Specify the ssh user.
`

func LoadConf(confFile string, args map[string]interface{}) error {
	conf, err := ini.LoadFile(confFile)
	if err != nil {
		return err
	}

	// PrivateKeyFile
	if args["--private-key-file"] == nil {
		tmpStr, ok := conf.Get("common", "PrivateKeyFile")
		if ok {
			args["--private-key-file"] = tmpStr
		} else {
			return errors.New("PrivateKeyFile is not specified")
		}
	}
	server.PrivateKeyFile = args["--private-key-file"].(string)

	// NBitsFile
	if args["--nBits-file"] == nil {
		tmpStr, ok := conf.Get("common", "NBitsFile")
		if ok {
			args["--nBits-file"] = tmpStr
		} else {
			return errors.New("NBitsFile is not specified")
		}
	}
	server.NBitsFile = args["--nBits-file"].(string)

	// ServerType
	if args["--server-type"] == nil {
		tmpStr, ok := conf.Get("common", "ServerType")
		if ok {
			args["--server-type"] = tmpStr
		} else {
			return errors.New("ServerType is not specified")
		}
	}
	server.ServerType = args["--server-type"].(string)

	// ServerPort
	if args["--server-port"] == nil {
		tmpStr, ok := conf.Get("common", "ServerPort")
		if ok {
			args["--server-port"] = tmpStr
		} else {
			return errors.New("ServerPort is not specified")
		}
	}
	server.ServerPort, err = strconv.Atoi(args["--server-port"].(string))
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
	server.LogFile = args["--log-file"].(string)
	if server.LogFile == "console" {
		server.LogWay = "console"
	} else {
		server.LogWay = "file"
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
	server.LogLevel = args["--log-level"].(string)

	// LogMaxDays
	if args["--log-max-days"] == nil {
		tmpStr, ok := conf.Get("common", "LogMaxDays")
		if ok {
			args["--log-max-days"] = tmpStr
		} else {
			return errors.New("LogMaxDays is not specified")
		}
	}
	server.LogMaxDays, err = strconv.Atoi(args["--log-max-days"].(string))
	if err != nil {
		return err
	}

	// HeartbeatTimeout
	if args["--heartbeat-timeout"] == nil {
		tmpStr, ok := conf.Get("common", "HeartbeatTimeout")
		if ok {
			args["--heartbeat-timeout"] = tmpStr
		} else {
			return errors.New("HeartbeatTimeout is not specified")
		}
	}
	server.HeartbeatTimeout, err = strconv.Atoi(args["--heartbeat-timeout"].(string))
	if err != nil {
		return err
	}

	// SshPort
	if args["--ssh-port"] == nil {
		tmpStr, ok := conf.Get("common", "SshPort")
		if ok {
			args["--ssh-port"] = tmpStr
		} else {
			args["--ssh-port"] = "0"
		}
	}
	server.SshPort, err = strconv.Atoi(args["--ssh-port"].(string))
	if err != nil {
		return err
	}

	// SshUser
	if args["--ssh-user"] == nil {
		tmpStr, ok := conf.Get("common", "SshUser")
		if ok {
			args["--ssh-user"] = tmpStr
		} else {
			args["--ssh-user"] = ""
		}
	}
	server.SshUser = args["--ssh-user"].(string)

	return err
}

func main() {
	// Parse arguments
	args := common.ParseArgs(&usage)
	if args == nil {
		return
	}

	// Load configuation
	err := LoadConf(args["--config-file"].(string), args)
	if err != nil {
		fmt.Printf("Error during loading configurations: %v\n", err)
		return
	}

	// Initialize conf
	err = server.InitConf()
	if err != nil {
		fmt.Printf("Error during initializing configurations: %v\n", err)
		return
	}

	// Start server
	server.Run()
}
