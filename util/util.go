package util

import (
	h "encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func ConvertHexToInt(hex string) (int64, error) {
	cleaned := strings.Replace(hex, "0x", "", -1)
	value, err := strconv.ParseInt(cleaned, 16, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func ConvertHexToBigInt(hex string) (*big.Int, error) {
	i := new(big.Int)
	hex = strings.Replace(hex, "0x", "", -1)
	n, ok := i.SetString(hex, 16)
	if !ok {
		return nil, fmt.Errorf("failed to convert hex to big int")
	}
	return n, nil
}

func ConvertHexToByte(hex string) ([]byte, error) {
	hex = strings.Replace(hex, "0x", "", -1)
	bytes, err := h.DecodeString(hex)
	if err != nil {
		return nil, err
	}
	return bytes, err
}
