package axtransport

import (
	"context"
	"encoding/binary"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
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
	binProcessor *AxBinProcessor
	handlerFunc  DataHandlerFunc
}

type AxTcpConnection struct {
	conn     net.Conn
	outChan  chan []byte
	ctx      context.Context
	cancelFn context.CancelFunc
}

func NewAxTcpConnection(ctx context.Context, conn net.Conn) *AxTcpConnection {
	res := &AxTcpConnection{
		conn:    conn,
		outChan: make(chan []byte, 100),
	}
	res.ctx, res.cancelFn = context.WithCancel(ctx)
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

func (a *AxTcpConnection) Write(data []byte) {
	if a.ctx.Err() != nil {
		return
	}
	a.outChan <- data
}

func NewAxTcp(ctx context.Context, logger zerolog.Logger, bind string, handlerFunc DataHandlerFunc) *AxTcp {
	res := &AxTcp{
		logger:      logger,
		parentCtx:   ctx,
		bind:        bind,
		handlerFunc: handlerFunc,
	}
	res.binProcessor = NewAxBinProcessor(logger).WithCompressionSize(1024)
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
	err := conn.SetReadDeadline(time.Now().Add(a.timeout))
	if err != nil {
		log.Error().Err(err).Msg("set read deadline failed")
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("set read deadline failed")
		return
	}
	axConn := NewAxTcpConnection(a.ctx, conn)
	defer axConn.Close()
	dataChannel := make(chan []byte)
	defer close(dataChannel)
	go func() {
		for {
			data, ok := <-dataChannel
			if !ok {
				log.Info().Msg("channel closed")
				return
			}
			data, err = a.binProcessor.Unmarshal(data)
			if err != nil {
				log.Error().Err(err).Msg("unmarshal failed")
				return
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
				axConn.Write(addSize32(rData))
			}(data)
		}
	}()
	err = a.readerTL(conn, log, dataChannel)
	if err != nil {
		log.Error().Err(err).Msg("read error")
	}
}

func (a *AxTcp) readerTL(conn net.Conn, log zerolog.Logger, dataChannel chan []byte) error {
	defer conn.Close()
	buf := make([]byte, 4096)
	var data []byte
	for {
		i, err := conn.Read(buf)
		if err != nil {
			log.Error().Err(err).Msg("fail to read")
			return err
		}
		if i == 0 {
			log.Warn().Err(err).Msg("connection closed")
			return errors.New("connection closed")
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Duration(a.timeout) * time.Millisecond))
		data = append(data, buf[:i]...)
		ix := 0
		for { // Нужен если получили 2-ва пакета вместе
			ix++
			ld := len(data)
			if ld >= 4 {
				l4 := getUInt32FromBytes(data[:4])
				if uint32(ld) >= l4+4 {
					dataChannel <- data[4 : l4+4]
					data = data[l4+4:]
				} else {
					break
				}
			} else {
				break
			}
		}
	}
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
