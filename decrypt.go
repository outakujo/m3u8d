package main

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

func AesDecryptByCBC(encrypted, key, iv []byte) (dst []byte, err error) {
	// 判断key长度
	keyLenMap := map[int]struct{}{16: {}, 24: {}, 32: {}}
	if _, ok := keyLenMap[len(key)]; !ok {
		err = errors.New("key长度必须是 16、24、32 其中一个")
		return
	}
	if iv != nil {
		if _, ok := keyLenMap[len(iv)]; !ok {
			err = errors.New("iv长度必须是 16、24、32 其中一个")
			return
		}
	}
	block, _ := aes.NewCipher(key)
	var blockMode cipher.BlockMode
	if iv != nil {
		blockMode = cipher.NewCBCDecrypter(block, iv)
	} else {
		blockSize := block.BlockSize()
		blockMode = cipher.NewCBCDecrypter(block, key[:blockSize])
	}
	dst = make([]byte, len(encrypted))
	// 解密
	blockMode.CryptBlocks(dst, encrypted)
	// 解码
	dst = PKCS7UNPadding(dst)
	return
}

func PKCS7UNPadding(originDataByte []byte) []byte {
	length := len(originDataByte)
	unpadding := int(originDataByte[length-1])
	return originDataByte[:(length - unpadding)]
}
