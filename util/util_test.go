package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConvertHexToInt(t *testing.T) {
	hex := "0x000000000000000000000000000000000000000000000000000000000000000d"
	value, err := ConvertHexToInt(hex)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), value)
}
