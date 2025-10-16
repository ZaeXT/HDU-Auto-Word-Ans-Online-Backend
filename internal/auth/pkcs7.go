package auth

import (
	"bytes"
	"crypto/aes"
)

// pkcs7Pad 对数据进行 PKCS7 填充
func pkcs7Pad(data []byte) []byte {
	blockSize := aes.BlockSize
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}
