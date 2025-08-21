package axtransport

import (
	"context"
	"net"
	"time"

	"github.com/rs/zerolog"
)

type AxTcpClient struct {
	conn         net.Conn
	address      string
	ctx          context.Context
	cancel       context.CancelFunc
	binProcessor *AxBinProcessor
	logger       zerolog.Logger
	timeout      time.Duration
	handlerFunc  DataHandlerFunc
}

func NewAxTcpClient(address string, secret []byte, ctx context.Context, logger zerolog.Logger) (*AxTcpClient, error) {
	res := &AxTcpClient{
		logger:  logger,
		address: address,
		ctx:     ctx,
		timeout: time.Second * 5,
	}
	res.binProcessor = NewAxBinProcessor(logger)
	if secret != nil {
		res.binProcessor.WithAES(secret)
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	res.conn = conn
	subCtx, cancel := context.WithCancel(ctx)
	res.cancel = cancel
	go res.readLoop(subCtx)
	return res, nil
}

func (a *AxTcpClient) Disconnect() error {
	if a.conn == nil {
		return nil // already disconnected
	}
	err := a.conn.Close()
	a.conn = nil
	if err != nil {
		return err
	}
	a.cancel()
	return nil
}

func (a *AxTcpClient) Connect() error {
	if a.conn != nil {
		return nil // already connected
	}

	conn, err := net.Dial("tcp", a.address)
	if err != nil {
		return err
	}
	a.conn = conn
	subCtx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	go a.readLoop(subCtx)
	return nil
}

func (a *AxTcpClient) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			sizeBytes, err := readNBytes(a.conn, 4)
			if err != nil {
				a.logger.Error().Err(err).Msg("failed to read header bytes")
				_ = a.Disconnect()
				return
			}
			if len(sizeBytes) < 4 {
				a.logger.Error().Msg("invalid size bytes received")
				_ = a.Disconnect()
				return
			}
			bodyLength := getUInt32FromBytes(sizeBytes)
			if bodyLength == 0 {
				a.logger.Error().Msg("failed to convert header to len")
				_ = a.Disconnect()
				break
			}
			if bodyLength > MaxBodySize {
				a.logger.Error().Uint32("body-length", bodyLength).Msg("body too big")
				_ = a.Disconnect()
				break
			}
			if err = a.conn.SetReadDeadline(time.Now().Add(a.timeout)); err != nil {
				a.logger.Error().Err(err).Msg("failed to set body read deadline")
				_ = a.Disconnect()
				return
			}
			dataBytes, err := readNBytes(a.conn, int(bodyLength))
			if err != nil {
				a.logger.Error().Err(err).Msg("failed to read body bytes")
				_ = a.Disconnect()
				return
			}
			inData, err := a.binProcessor.Unmarshal(dataBytes)
			if err != nil {
				a.logger.Error().Err(err).Msg("unmarshal failed")
				_ = a.Disconnect()
				break
			}
			startTime := time.Now()
			outData, err := a.handlerFunc(inData, ctx)
			if err != nil {
				a.logger.Error().Err(err).Msg("handle request failed")
				_ = a.Disconnect()
				return
			}
			outData, err = a.binProcessor.Marshal(outData)
			if err != nil {
				a.logger.Error().Err(err).Msg("marshal failed")
				_ = a.Disconnect()
				return
			}
			if outData != nil {
				if err = a.conn.SetWriteDeadline(time.Now().Add(a.timeout)); err != nil {
					a.logger.Error().Err(err).Msg("failed to set write deadline")
					_ = a.Disconnect()
					return
				}
				if err = a.Send(outData); err != nil {
					a.logger.Error().Err(err).Msg("failed to write response")
					_ = a.Disconnect()
					return
				}
			}
			endTime := time.Now()
			deltaTime := endTime.Sub(startTime)
			if a.logger.GetLevel() <= zerolog.DebugLevel && deltaTime > time.Millisecond*250 {
				a.logger.Debug().
					Str("address", a.address).
					Str("duration", endTime.Sub(startTime).String()).
					Msg("TCP response sent")
			}
		}
	}
}

func (a *AxTcpClient) IsConnected() bool {
	if a.conn == nil {
		return false
	}
	if _, err := a.conn.Write([]byte{}); err != nil {
		return false
	}
	return true
}

func (a *AxTcpClient) Send(in []byte) error {

	inBts, err := a.binProcessor.Marshal(in)
	if err != nil {
		return err
	}
	if err = a.conn.SetWriteDeadline(time.Now().Add(a.timeout)); err != nil {
		return err
	}
	_, err = a.conn.Write(addSize32(inBts))
	if err != nil {
		return err
	}
	return nil
}
