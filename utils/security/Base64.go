package security

import "encoding/base64"

func Base64Encoding(text []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(text))
}

func Base64Decoding(text []byte) ([]byte, error) {
	decodeBytes, err := base64.StdEncoding.DecodeString(string(text))
	return decodeBytes, err
}
