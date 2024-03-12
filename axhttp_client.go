package axtransport

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"time"
)

/*
 __    _           ___
|  |  |_|_____ ___|_  |
|  |__| |     | .'|  _|
|_____|_|_|_|_|__,|___|
zed (12.03.2024)
*/

type AxHttpClient struct {
	client       http.Client
	binProcessor *AxBinProcessor
}

func NewAxHttpClient(secret []byte) *AxHttpClient {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res := &AxHttpClient{
		client: client,
	}
	res.binProcessor = NewAxBinProcessor(zerolog.Nop())
	if secret != nil {
		res.binProcessor.WithAES(secret)
	}
	return res
}

func (a *AxHttpClient) Post(url string, data []byte) ([]byte, error) {
	var err error
	data, err = a.binProcessor.Marshal(data)
	if err != nil {
		return nil, errors.New("fail to marshal:" + err.Error())
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/octet-stream")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("bad status code (%d) %s", resp.StatusCode, resp.Status))
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	data, err = a.binProcessor.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
