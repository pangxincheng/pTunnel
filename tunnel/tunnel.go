package tunnel

import (
	"bufio"
	"fmt"
	"io"
	"pTunnel/conn"
	"pTunnel/utils/security"
	"sync"
)

func ClientTunnelSafetyCheck(tunnel conn.Socket, secretKey []byte) bool {
	bytes, err := security.AESEncryptBase64(secretKey, secretKey)
	if err != nil {
		return false
	}
	err = tunnel.WriteLine(bytes)
	return err == nil
}

func ServerTunnelSafetyCheck(tunnel conn.Socket, secretKey []byte) bool {
	bytes, err := tunnel.ReadLine()
	if err != nil {
		return false
	}
	bytes, err = security.AESDecryptBase64(bytes, secretKey)
	return err == nil && string(bytes) == string(secretKey)
}

func UnsafeTunnel(request conn.Socket, worker conn.Socket) {
	var wait sync.WaitGroup
	pipe := func(src conn.Socket, dst conn.Socket) {
		defer wait.Done()
		_, err := io.Copy(src, dst)
		if err != nil {
			return
		}
	}
	wait.Add(2)
	go pipe(request, worker)
	go pipe(worker, request)
	wait.Wait()
}

func SafeTunnel(request conn.Socket, worker conn.Socket, secretKey []byte) {
	var wait sync.WaitGroup

	encryptPipe := func(src conn.Socket, dst conn.Socket, key []byte) {
		defer wait.Done()
		reader := bufio.NewReader(src)
		buf := make([]byte, 10*1024)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				return
			}
			fmt.Println(buf[:n])
			bytes, err := security.AESEncryptBase64(buf[:n], key)
			if err != nil {
				return
			}
			err = dst.WriteLine(bytes)
			if err != nil {
				return
			}
		}
	}

	decryptPipe := func(src conn.Socket, dst conn.Socket, key []byte) {
		defer wait.Done()
		for {
			bytes, err := src.ReadLine()
			if err != nil {
				return
			}
			bytes, err = security.AESDecryptBase64(bytes, key)
			fmt.Println(bytes)
			if err != nil {
				return
			}
			_, err = dst.Write(bytes)
			if err != nil {
				return
			}
		}
	}

	wait.Add(2)
	go encryptPipe(request, worker, secretKey)
	go decryptPipe(worker, request, secretKey)
	wait.Wait()
}
