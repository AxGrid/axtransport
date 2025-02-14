package axtransport

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
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
	httpApiPath           string
	httpConnectionTimeout time.Duration
	httpEventTimeout      time.Duration
	tcpServerHost         string
	tcpServerPort         int
	tcpWriteBufSize       int
	tcpConnectionTimeout  time.Duration
	dataHandlerFunc       DataHandlerFunc
	binProcessor          BinProcessor
	ctx                   context.Context
	aesSecret             []byte
	compressionSize       int
	logger                zerolog.Logger
	chiRouter             chi.Router
}

func AxTransport() *Builder {
	return &Builder{
		httpServerHost:        "0.0.0.0",
		httpServerPort:        0,
		httpApiPath:           "/api",
		httpConnectionTimeout: 5 * time.Second,
		httpEventTimeout:      50 * time.Second,
		tcpServerHost:         "0.0.0.0",
		tcpServerPort:         0,
		tcpWriteBufSize:       200,
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

func (b *Builder) WithCPWriteBufSize(size int) *Builder {
	b.tcpWriteBufSize = size
	return b
}

func (b *Builder) WithHTTPApiPath(path string) *Builder {
	b.httpApiPath = path
	return b
}

func (b *Builder) WithHTTPEventTimeout(timeout time.Duration) *Builder {
	b.httpEventTimeout = timeout
	return b
}

func (b *Builder) WithCompressionSize(size int) *Builder {
	b.compressionSize = size
	return b
}

func (b *Builder) WithHTTPRouter(r chi.Router) *Builder {
	b.chiRouter = r
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

func (b *Builder) WithCustomBinProcessor(processor BinProcessor) *Builder {
	b.binProcessor = processor
	return b
}

func (b *Builder) Build() *Transport {
	res := &Transport{
		b: b,
	}
	if b.binProcessor == nil {
		b.binProcessor = NewAxBinProcessor(b.logger)
	}
	if b.httpServerPort != 0 {
		res.http = NewAxHttp(b.ctx, b.logger, fmt.Sprintf("%s:%d", b.httpServerHost, b.httpServerPort), "/api", b.binProcessor, b.dataHandlerFunc)
		if b.chiRouter != nil {
			res.http.WithRouter(b.chiRouter)
		}
		b.logger.Debug().Str("api-path", b.httpApiPath).Str("bind", res.http.bind).Msg("http server created")
		if b.httpConnectionTimeout != 0 {
			res.http.WithTimeout(b.httpConnectionTimeout)
		}
		if b.aesSecret != nil {
			res.http.WithAES(b.aesSecret)
		}
		res.http.WithCompressionSize(b.compressionSize)
	}
	if b.tcpServerPort != 0 {
		res.tcp = NewAxTcp(b.ctx, b.logger, fmt.Sprintf("%s:%d", b.tcpServerHost, b.tcpServerPort), b.tcpWriteBufSize, b.binProcessor, b.dataHandlerFunc)
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
