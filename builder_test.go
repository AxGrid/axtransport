package axtransport

import (
	"github.com/axgrid/axtransport/internal"
	"testing"
)

type customBin struct {
	aes *internal.AES
}

func (c *customBin) WithAES(secretKey []byte) BinProcessor {
	c.aes = internal.NewAES(secretKey)
	return c
}

func (c *customBin) WithCompressionSize(size int) BinProcessor {
	return c
}

func (c *customBin) Unmarshal(in []byte) ([]byte, error) {
	return in, nil
}

func (c *customBin) Marshal(in []byte) ([]byte, error) {
	return in, nil
}

func TestBuilder_Build(t *testing.T) {
	b := &customBin{}
	key := []byte("12345678901234567890123456789012")
	a := AxTransport().
		WithHTTPServer("localhost", 8000).
		WithTCPServer("localhost", 8001).
		WithCustomBinProcessor(b).
		WithAES(key).
		Build()
	_, ok := a.http.binProcessor.(*customBin)
	if !ok {
		panic("not customBin in http")
	}
	_, ok = a.tcp.binProcessor.(*customBin)
	if !ok {
		panic("not customBin in tcp")
	}
}
