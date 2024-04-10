package security

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"os"
	"pTunnel/utils/common"
	"strconv"
)

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf)
	}
	return chunks
}

func MarshalPKCS8PrivateKey(key *rsa.PrivateKey) []byte {
	info := struct {
		Version             int
		PrivateKeyAlgorithm []asn1.ObjectIdentifier
		PrivateKey          []byte
	}{}
	info.Version = 0
	info.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 1)
	info.PrivateKeyAlgorithm[0] = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	info.PrivateKey = x509.MarshalPKCS1PrivateKey(key)

	k, _ := asn1.Marshal(info)
	return k
}

func RSAGenKey(keyLength int, saveDir string) error {
	if err := common.Mkdir(saveDir, true); err != nil {
		return err
	}

	CloseWriter := func(writer *os.File) {
		_ = writer.Close()
	}

	// Create private key and public key files
	privateKeyWriter, err := os.Create(saveDir + "/PrivateKey.pem")
	if err != nil {
		return err
	}
	defer CloseWriter(privateKeyWriter)
	publicKeyWriter, err := os.Create(saveDir + "/PublicKey.pem")
	if err != nil {
		return err
	}
	defer CloseWriter(publicKeyWriter)
	nBitsWriter, err := os.Create(saveDir + "/NBits.txt")
	if err != nil {
		return err
	}
	defer CloseWriter(nBitsWriter)

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, keyLength)
	if err != nil {
		return err
	}
	derStream := MarshalPKCS8PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derStream,
	}
	err = pem.Encode(privateKeyWriter, block)
	if err != nil {
		return err
	}

	// Generate public key
	publicKey := &privateKey.PublicKey
	derPkix, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}
	block = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derPkix,
	}
	err = pem.Encode(publicKeyWriter, block)
	if err != nil {
		return err
	}

	// Save n bits
	_, err = nBitsWriter.WriteString(strconv.Itoa(keyLength))
	if err != nil {
		return err
	}

	return nil
}

func RSAEncrypt(data []byte, publicKey []byte, nBits int) ([]byte, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("public key error")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	partLen := nBits/8 - 11
	chunks := split(data, partLen)
	buffer := bytes.NewBufferString("")
	for _, chunk := range chunks {
		encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pub.(*rsa.PublicKey), chunk)
		if err != nil {
			return []byte(""), err
		}
		buffer.Write(encrypted)
	}
	return buffer.Bytes(), nil
}

func RSADecrypt(data []byte, privateKey []byte, nBits int) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return []byte(""), errors.New("private key error")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return []byte(""), err
	}

	partLen := nBits / 8
	chunks := split(data, partLen)
	buffer := bytes.NewBufferString("")
	for _, chunk := range chunks {
		decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, priv.(*rsa.PrivateKey), chunk)
		if err != nil {
			return []byte(""), err
		}
		buffer.Write(decrypted)
	}
	return buffer.Bytes(), nil
}

// RSAEncryptBase64 encrypts data with public key and returns base64 encoded string
func RSAEncryptBase64(data []byte, publicKey []byte, nBits int) ([]byte, error) {
	data, err := RSAEncrypt(data, publicKey, nBits)
	if err != nil {
		return nil, err
	}
	return Base64Encoding(data), nil
}

// RSADecryptBase64 decrypts base64 encoded data with private key
func RSADecryptBase64(data []byte, privateKey []byte, nBits int) ([]byte, error) {
	data, err := Base64Decoding(data)
	if err != nil {
		return nil, err
	}
	return RSADecrypt(data, privateKey, nBits)
}
