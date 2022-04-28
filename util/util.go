package util

import (
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
	h := strings.Replace(hex, "0x", "", -1)
	n, ok := i.SetString(h, 16)
	if !ok {
		return nil, fmt.Errorf("failed to convert hex to big int")
	}
	return n, nil
}
