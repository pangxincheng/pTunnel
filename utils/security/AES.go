package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"github.com/thanhpk/randstr"
)

func PKCS7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func PKCS7UnPadding(data []byte) []byte {
	length := len(data)
	unPadding := int(data[length-1])
	return data[:(length - unPadding)]
}

func AESEncrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	data = PKCS7Padding(data, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	encrypted := make([]byte, len(data))
	blockMode.CryptBlocks(encrypted, data)
	return encrypted, err
}

func AESDecrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	decrypted := make([]byte, len(data))
	blockMode.CryptBlocks(decrypted, data)
	decrypted = PKCS7UnPadding(decrypted)
	return decrypted, err
}

// AESEncryptBase64 : AES encrypt -> Base64 encode
func AESEncryptBase64(data []byte, key []byte) ([]byte, error) {
	encrypted, err := AESEncrypt(data, key)
	if err != nil {
		return nil, err
	}
	return Base64Encoding(encrypted), nil
}

// AESDecryptBase64 : Base64 decode -> AES decrypt
func AESDecryptBase64(data []byte, key []byte) ([]byte, error) {
	data, err := Base64Decoding(data)
	if err != nil {
		return nil, err
	}
	data, err = AESDecrypt(data, key)
	if err != nil {
		return nil, err
	}
	return data, err
}

func AesGenKey(length int) []byte {
	return []byte(randstr.String(length))
}
