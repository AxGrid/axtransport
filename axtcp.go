package axtransport

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"net"
	"time"
)

type AxTcp struct {
	logger       zerolog.Logger
	parentCtx    context.Context
	ctx          context.Context
	cancelFn     context.CancelFunc
	timeout      time.Duration
	bind         string
	writeBufSize int
	listener     net.Listener
	binProcessor BinProcessor
	handlerFunc  DataHandlerFunc
}

type AxTcpConnection struct {
	logger   zerolog.Logger
	conn     net.Conn
	outSize  int
	outChan  chan []byte
	ctx      context.Context
	cancelFn context.CancelFunc
}

func NewAxTcpConnection(ctx context.Context, logger zerolog.Logger, conn net.Conn, outSize int) *AxTcpConnection {
	res := &AxTcpConnection{
		logger:  logger,
		conn:    conn,
		outSize: outSize,
		outChan: make(chan []byte, outSize),
	}
	res.ctx, res.cancelFn = context.WithCancel(ctx)
	res.ctx = context.WithValue(res.ctx, "connection", res)
	go func() {
		defer conn.Close()
		for {
			select {
			case <-res.ctx.Done():
				close(res.outChan)
				return
			case data, ok := <-res.outChan:
				if !ok {
					return
				}
				_, err := conn.Write(addSize32(data))
				if err != nil {
					res.logger.Error().Err(err).Msg("can't write to connection")
					return
				}
			}
		}
	}()
	return res
}

func (a *AxTcpConnection) Close() {
	a.cancelFn()
}

var (
	ErrTooMuchData = errors.New("too much data in out chan")
)

func (a *AxTcpConnection) Write(data []byte) error {
	if len(a.outChan) > a.outSize/2 {
		a.logger.Error().Err(a.ctx.Err()).Msg("too much data in out chan")
		return ErrTooMuchData
	}
	if a.ctx.Err() != nil {
		a.logger.Error().Err(a.ctx.Err()).Msg("can't write to connection out chan")
		return a.ctx.Err()
	}
	a.outChan <- data
	return nil
}

func (a *AxTcpConnection) Read(b []byte) (n int, err error) {
	return a.conn.Read(b)
}

func (a *AxTcpConnection) SetReadDeadline(t time.Time) error {
	return a.conn.SetReadDeadline(t)
}

func (a *AxTcpConnection) SetWriteDeadline(t time.Time) error {
	return a.conn.SetWriteDeadline(t)
}

func NewAxTcp(ctx context.Context, logger zerolog.Logger, bind string, writeBufSize int, bin BinProcessor, handlerFunc DataHandlerFunc) *AxTcp {
	res := &AxTcp{
		logger:       logger,
		parentCtx:    ctx,
		bind:         bind,
		writeBufSize: writeBufSize,
		handlerFunc:  handlerFunc,
	}
	res.binProcessor = bin.WithCompressionSize(1024) //NewAxBinProcessor(logger).WithCompressionSize(1024)
	return res
}

func (a *AxTcp) WithAES(secretKey []byte) *AxTcp {
	a.binProcessor.WithAES(secretKey)
	return a
}

func (a *AxTcp) WithCompressionSize(size int) *AxTcp {
	a.binProcessor.WithCompressionSize(size)
	return a
}

func (a *AxTcp) WithTimeout(timeout time.Duration) *AxTcp {
	a.timeout = timeout
	return a
}

func (a *AxTcp) WithWriteBUfSize(size int) *AxTcp {
	a.writeBufSize = size
	return a
}

func (a *AxTcp) Start() error {
	var err error
	a.ctx, a.cancelFn = context.WithCancel(a.parentCtx)
	a.logger.Debug().Str("bind", a.bind).Msg("start tcp server")
	a.listener, err = net.Listen("tcp", a.bind)
	if err != nil {
		return err
	}
	go a.listen()
	return nil
}

func (a *AxTcp) Stop() {
	a.cancelFn()
	_ = a.listener.Close()
}

func (a *AxTcp) listen() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			conn, err := a.listener.Accept()
			if err != nil && a.ctx.Err() == nil {
				a.logger.Warn().Err(err).Msg("accept failed")
				continue
			} else if err != nil {
				return
			}
			go a.handleConn(conn)
		}
	}
}

func (a *AxTcp) handleConn(conn net.Conn) {
	log := a.logger.With().Str("remote", conn.RemoteAddr().String()).Logger()
	defer conn.Close()
	axConn := NewAxTcpConnection(a.ctx, a.logger, conn, a.writeBufSize)
	timeout := time.Duration(a.timeout) * time.Millisecond
	defer axConn.Close()
	for {
		err := conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			log.Error().Err(err).Msg("set read deadline failed")
			axConn.Write([]byte(err.Error()))
			return
		}
		sizeBytes, err := readNBytes(conn, 4)
		if err != nil {
			log.Error().Err(err).Msg("failed to read header bytes")
			axConn.Write([]byte(err.Error()))
			break
		}
		bodyLength := getUInt32FromBytes(sizeBytes)
		if bodyLength == 0 {
			log.Error().Msg("failed to convert header to len")
			axConn.Write([]byte(err.Error()))
			break
		}
		err = conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			log.Error().Err(err).Msg("failed to set body read deadline")
			axConn.Write([]byte(err.Error()))
			break
		}
		dataBytes, err := readNBytes(conn, int(bodyLength))
		if err != nil {
			log.Error().Err(err).Msg("failed to read body bytes")
			axConn.Write([]byte(err.Error()))
			break
		}
		data, err := a.binProcessor.Unmarshal(dataBytes)
		if err != nil {
			log.Error().Err(err).Msg("unmarshal failed")
			axConn.Write([]byte(err.Error()))
			break
		}
		go func(rData []byte) {
			rData, err = a.handlerFunc(rData, axConn.ctx)
			if err != nil {
				log.Error().Err(err).Msg("handle request failed")
				axConn.Write([]byte(err.Error()))
				axConn.Close()
				return
			}
			rData, err = a.binProcessor.Marshal(rData)
			if err != nil {
				log.Error().Err(err).Msg("marshal failed")
				axConn.Write([]byte(err.Error()))
				axConn.Close()
				return
			}
			axConn.Write(rData)
		}(data)
	}
}

func readNBytes(conn net.Conn, n int) ([]byte, error) {
	buff := make([]byte, n)
	nRead, err := io.ReadFull(conn, buff)
	if err != nil {
		return nil, err
	}
	if nRead != n {
		return buff, fmt.Errorf("failed to read packet in full")
	}

	return buff, nil
}

func getUInt32FromBytes(lens []byte) uint32 {
	return binary.LittleEndian.Uint32(lens)
}

func addSize32(data []byte) []byte {
	ld := uint32(len(data))
	return append(getBytesFromUInt32(ld), data...)
}

func getBytesFromUInt32(len uint32) []byte {
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, len)
	return bs
}
