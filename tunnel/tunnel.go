package tunnel

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"pTunnel/conn"
	"pTunnel/utils/log"
	"pTunnel/utils/security"
	"pTunnel/utils/serialize"
	"sync"
)

func constructSafetyMsg(secretKey []byte) ([]byte, error) {
	dict := make(map[string]interface{})
	dict["SecretKey"] = string(secretKey)
	dict["Salt"] = string(md5.New().Sum(nil))
	bytes, err := serialize.Serialize(&dict)
	if err != nil {
		return nil, err
	}
	bytes, err = security.AESEncryptBase64(bytes, secretKey)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func verifySafetyMsg(msg []byte, secretKey []byte) bool {
	bytes, err := security.AESDecryptBase64(msg, secretKey)
	if err != nil {
		return false
	}
	dict := make(map[string]interface{})
	err = serialize.Deserialize(bytes, &dict)
	return err == nil && dict["SecretKey"].(string) == string(secretKey)
}

func ClientTunnelSafetyCheck(tunnel conn.Socket, secretKey []byte) bool {
	bytes, err := constructSafetyMsg(secretKey)
	if err != nil {
		return false
	}
	fmt.Println("write safety msg")
	err = tunnel.WriteLine(bytes)
	if err != nil {
		return false
	}
	fmt.Println("read safety msg")
	bytes, err = tunnel.ReadLine()
	if err != nil {
		return false
	}
	return verifySafetyMsg(bytes, secretKey)
}

func ServerTunnelSafetyCheck(tunnel conn.Socket, secretKey []byte) bool {
	bytes, err := tunnel.ReadLine()
	if err != nil {
		return false
	}
	if !verifySafetyMsg(bytes, secretKey) {
		return false
	}
	bytes, err = constructSafetyMsg(secretKey)
	if err != nil {
		return false
	}
	err = tunnel.WriteLine(bytes)
	return err == nil
}

func UnsafeTunnel(request conn.Socket, worker conn.Socket) {
	var wait sync.WaitGroup
	log.Debug("Tunnel start")
	pipe := func(src conn.Socket, dst conn.Socket) {
		defer request.Close()
		defer worker.Close()
		defer wait.Done()
		buf := make([]byte, 10*1024)
		for {
			n, err := src.Read(buf)
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
			_, err = dst.Write(buf[:n])
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
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
		defer request.Close()
		defer worker.Close()
		defer wait.Done()
		reader := bufio.NewReader(src)
		buf := make([]byte, 10*1024)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
			bytes, err := security.AESEncryptBase64(buf[:n], key)
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
			err = dst.WriteLine(bytes)
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
		}
	}

	decryptPipe := func(src conn.Socket, dst conn.Socket, key []byte) {
		defer request.Close()
		defer worker.Close()
		defer wait.Done()
		for {
			bytes, err := src.ReadLine()
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
			bytes, err = security.AESDecryptBase64(bytes, key)
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
			_, err = dst.Write(bytes)
			if err != nil {
				log.Debug("Tunnel pipe error: %v", err)
				return
			}
		}
	}

	wait.Add(2)
	go encryptPipe(request, worker, secretKey)
	go decryptPipe(worker, request, secretKey)
	wait.Wait()
}
