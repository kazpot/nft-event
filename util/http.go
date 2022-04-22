package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

func GetRequest(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if err = res.Body.Close(); err != nil {
		return nil, err
	}

	return body, nil
}

func PostRequest(url string, contentType string, data []byte) ([]byte, error) {
	res, err := http.Post(url, contentType, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("post error %w", err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed ot read body %w", err)
	}

	if err = res.Body.Close(); err != nil {
		return nil, err
	}

	return body, nil
}
