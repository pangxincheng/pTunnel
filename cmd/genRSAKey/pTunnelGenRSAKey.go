package main

import (
	"fmt"
	"os"
	"pTunnel/utils/common"
	"pTunnel/utils/security"
	"pTunnel/utils/version"

	"github.com/docopt/docopt-go"
)

var usage = `pTunnelGenRSAKey is a tool to generate RSA key pair.
Usage:
	pTunnelGenRSAKey [options]

Options:
	-h --help              Show help information in screen.
	--version              Show version.
	-l --length=<length>   Specify the length of RSA key pair [default: 2048].
	-d --dir=<dir>         Specify the directory to save the key pair [default: ./cert].
`

func main() {
	opts, err := docopt.ParseArgs(usage, os.Args[1:], version.GetVersion())
	if err != nil {
		fmt.Printf("Error during parsing arguments: %s\n", err.Error())
		return
	}

	length, err := opts.Int("--length")
	if err != nil {
		fmt.Printf("Error during parsing length: %s\n", err.Error())
		return
	}

	saveDir, err := opts.String("--dir")
	if err != nil {
		fmt.Printf("Error during parsing saveDir: %s\n", err.Error())
		return
	}

	err = common.Mkdir(saveDir, true)
	if err != nil {
		fmt.Printf("Error during creating directory: %s\n", err.Error())
		return
	}

	err = security.RSAGenKey(length, saveDir)
	if err != nil {
		fmt.Printf("Error during generating RSA key pair: %s\n", err.Error())
		return
	}
}
