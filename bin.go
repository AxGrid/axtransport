package axtransport

import (
	"errors"
	"github.com/axgrid/axtransport/internal"
	"github.com/axgrid/axtransport/protobuf"
	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
)

/*
	__    _           ___

|  |  |_|_____ ___|_  |
|  |__| |     | .'|  _|
|_____|_|_|_|_|__,|___|
zed (12.03.2024)
*/

type BinProcessor interface {
	WithAES(secretKey []byte) BinProcessor
	WithCompressionSize(size int) BinProcessor
	Unmarshal(in []byte) ([]byte, error)
	Marshal(in []byte) ([]byte, error)
}

var ErrNoAES = errors.New("no aes")

type AxBinProcessor struct {
	logger          zerolog.Logger
	aes             *internal.AES
	compressionSize int
}

func NewAxBinProcessor(logger zerolog.Logger) *AxBinProcessor {
	return &AxBinProcessor{
		logger: logger,
	}
}

func (b *AxBinProcessor) WithAES(secretKey []byte) BinProcessor {
	b.aes = internal.NewAES(secretKey)
	return b
}

func (b *AxBinProcessor) WithCompressionSize(size int) BinProcessor {
	b.compressionSize = size
	return b
}

func (b *AxBinProcessor) Unmarshal(in []byte) ([]byte, error) {
	var pck protobuf.PPacket
	err := proto.Unmarshal(in, &pck)
	if err != nil {
		return nil, err
	}
	switch pck.Encryption {
	case protobuf.PEncryption_P_ENCRYPTION_AES:
		if b.aes == nil {
			return nil, ErrNoAES
		}
		pck.Payload, err = b.aes.Decrypt(pck.Payload)
		if err != nil {
			return nil, err
		}
	}
	switch pck.Compression {
	case protobuf.PCompression_P_COMPRESSION_GZIP:
		pck.Payload, err = internal.GUnzipData(pck.Payload)
		if err != nil {
			return nil, err
		}
	}
	return pck.Payload, nil
}

func (b *AxBinProcessor) Marshal(in []byte) ([]byte, error) {
	var pck protobuf.PPacket
	var err error
	pck.Payload = in
	if b.compressionSize > 0 && len(in) > b.compressionSize {
		pck.Compression = protobuf.PCompression_P_COMPRESSION_GZIP
		pck.Payload, err = internal.GZipData(pck.Payload)
		if err != nil {
			return nil, err
		}
	}
	if b.aes != nil {
		pck.Encryption = protobuf.PEncryption_P_ENCRYPTION_AES
		pck.Payload, err = b.aes.Encrypt(pck.Payload)
		if err != nil {
			return nil, err
		}
	}
	return proto.Marshal(&pck)
}
