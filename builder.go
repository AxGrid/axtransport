package axtransport

import (
	"context"
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

	tcpConnectionTimeout time.Duration
	dataHandlerFunc      DataHandlerFunc
	ctx                  context.Context
	logger               zerolog.Logger
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

func (b *Builder) WithDataHandlerFunc(dataHandlerFunc DataHandlerFunc) *Builder {
	b.dataHandlerFunc = dataHandlerFunc
	return b
}
