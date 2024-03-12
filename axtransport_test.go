package axtransport

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAxTransportHttpRequest(t *testing.T) {

	f := func(d []byte, ctx context.Context) ([]byte, error) {
		log.Debug().Msgf("data: %s", string(d))
		return d, nil
	}
	key := []byte("12345678901234567890123456789012")
	log.Debug().Msgf("aes-test-key: %s", string(key))

	transport := AxTransport().WithHTTPServer("", 8081).WithLogger(log.With().Logger()).WithAES(key).WithDataHandlerFunc(f).Build()
	assert.Nil(t, transport.Start())
	defer transport.Stop()
	client := NewAxHttpClient(key)
	data, err := client.Post("http://localhost:8081/api", []byte("test-string"))
	assert.Nil(t, err)
	assert.Equal(t, "test-string", string(data))

}
