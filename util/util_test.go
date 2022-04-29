package util

import (
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestConvertHexToInt(t *testing.T) {
	hex := "0x000000000000000000000000000000000000000000000000000000000000000d"
	value, err := ConvertHexToInt(hex)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), value)
}

func TestConvertHexToBigInt(t *testing.T) {
	hex := "0x000000000000000000000000000000000000000000000000000000000000000d"
	value, err := ConvertHexToBigInt(hex)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(13), value)
}

func TestConvertHexToByte(t *testing.T) {
	hex := "0x80ac58cd"
	value, err := ConvertHexToByte(hex)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x80, 0xac, 0x58, 0xcd}, value)
}
