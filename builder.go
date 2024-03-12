package axtransport

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"time"
)

/*
 __    _           ___
|  |  |_|_____ ___|_  |
|  |__| |     | .'|  _|
|_____|_|_|_|_|__,|___|
zed (12.03.2024)
*/

type Builder struct {
	httpServerHost        string
	httpServerPort        int
	httpConnectionTimeout time.Duration
	httpEventTimeout      time.Duration
	tcpServerHost         string
	tcpServerPort         int
	tcpConnectionTimeout  time.Duration
	dataHandlerFunc       DataHandlerFunc
	ctx                   context.Context
	aesSecret             []byte
	compressionSize       int
	logger                zerolog.Logger
}

func AxTransport() *Builder {
	return &Builder{
		httpServerHost:        "0.0.0.0",
		httpServerPort:        0,
		httpConnectionTimeout: 5 * time.Second,
		httpEventTimeout:      50 * time.Second,
		tcpServerHost:         "0.0.0.0",
		tcpServerPort:         0,
		tcpConnectionTimeout:  30 * time.Second,
		logger:                zerolog.Nop(),
		ctx:                   context.Background(),
		compressionSize:       1024,
	}
}

func (b *Builder) WithHTTPServer(host string, port int) *Builder {
	b.httpServerHost = host
	b.httpServerPort = port
	return b
}

func (b *Builder) WithTCPServer(host string, port int) *Builder {
	b.tcpServerHost = host
	b.tcpServerPort = port
	return b
}

func (b *Builder) WithHTTPConnectionTimeout(timeout time.Duration) *Builder {
	b.httpConnectionTimeout = timeout
	return b
}

func (b *Builder) WithTCPConnectionTimeout(timeout time.Duration) *Builder {
	b.tcpConnectionTimeout = timeout
	return b
}

func (b *Builder) WithHTTPEventTimeout(timeout time.Duration) *Builder {
	b.httpEventTimeout = timeout
	return b
}

func (b *Builder) WithLogger(logger zerolog.Logger) *Builder {
	b.logger = logger
	return b
}

func (b *Builder) WithContext(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}

func (b *Builder) WithAES(secretKey []byte) *Builder {
	b.aesSecret = secretKey
	return b
}

func (b *Builder) WithDataHandlerFunc(dataHandlerFunc DataHandlerFunc) *Builder {
	b.dataHandlerFunc = dataHandlerFunc
	return b
}

func (b *Builder) Build() *Transport {
	res := &Transport{
		b: b,
	}
	if b.httpServerPort != 0 {
		res.http = NewAxHttp(b.ctx, b.logger, fmt.Sprintf("%s:%d", b.httpServerHost, b.httpServerPort), "/api", b.dataHandlerFunc)
		b.logger.Debug().Str("api-path", "/api").Str("bind", res.http.bind).Msg("http server created")
		if b.httpConnectionTimeout != 0 {
			res.http.WithTimeout(b.httpConnectionTimeout)
		}
		if b.aesSecret != nil {
			res.http.WithAES(b.aesSecret)
		}
		res.http.WithCompressionSize(b.compressionSize)
	}
	if b.tcpServerPort != 0 {
		res.tcp = NewAxTcp(b.ctx, b.logger, fmt.Sprintf("%s:%d", b.tcpServerHost, b.tcpServerPort), b.dataHandlerFunc)
		if b.tcpConnectionTimeout != 0 {
			res.tcp.WithTimeout(b.tcpConnectionTimeout)
		}
		if b.aesSecret != nil {
			res.tcp.WithAES(b.aesSecret)
		}
		res.tcp.WithCompressionSize(b.compressionSize)
	}
	return res
}
