package axtransport

import (
	"context"
	"encoding/binary"
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
	listener     net.Listener
	binProcessor BinProcessor
	handlerFunc  DataHandlerFunc
}

type AxTcpConnection struct {
	logger   zerolog.Logger
	conn     net.Conn
	outChan  chan []byte
	ctx      context.Context
	cancelFn context.CancelFunc
}

func NewAxTcpConnection(ctx context.Context, logger zerolog.Logger, conn net.Conn) *AxTcpConnection {
	res := &AxTcpConnection{
		logger:  logger,
		conn:    conn,
		outChan: make(chan []byte, 100),
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

func (a *AxTcpConnection) Write(data []byte) error {
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

func NewAxTcp(ctx context.Context, logger zerolog.Logger, bind string, bin BinProcessor, handlerFunc DataHandlerFunc) *AxTcp {
	res := &AxTcp{
		logger:      logger,
		parentCtx:   ctx,
		bind:        bind,
		handlerFunc: handlerFunc,
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
	axConn := NewAxTcpConnection(a.ctx, a.logger, conn)
	timeout := time.Duration(a.timeout) * time.Millisecond
	defer axConn.Close()
	for {
		err := conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			log.Error().Err(err).Msg("set read deadline failed")
			return
		}
		sizeBytes, err := readNBytes(conn, 4)
		if err != nil {
			log.Error().Err(err).Msg("failed to read header bytes")
			break
		}
		bodyLength := getUInt32FromBytes(sizeBytes)
		if bodyLength == 0 {
			log.Error().Msg("failed to convert header to len")
			break
		}
		err = conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			log.Error().Err(err).Msg("failed to set body read deadline")
			break
		}
		dataBytes, err := readNBytes(conn, int(bodyLength))
		if err != nil {
			log.Error().Err(err).Msg("failed to read body bytes")
			break
		}
		data, err := a.binProcessor.Unmarshal(dataBytes)
		if err != nil {
			log.Error().Err(err).Msg("unmarshal failed")
			break
		}
		go func(rData []byte) {
			rData, err = a.handlerFunc(rData, axConn.ctx)
			if err != nil {
				log.Error().Err(err).Msg("handle request failed")
				axConn.Close()
				return
			}
			rData, err = a.binProcessor.Marshal(rData)
			if err != nil {
				log.Error().Err(err).Msg("marshal failed")
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
