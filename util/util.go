package util

import (
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
