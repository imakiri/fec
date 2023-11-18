package fec

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/klauspost/reedsolomon"
	"io"
)

const Addr = 1

type Encoder struct {
	rs    reedsolomon.Encoder
	total uint64
	data  uint64
	csn   uint64

	resultQueue chan []byte
}

func NewEncoder(dataParts, totalParts uint64) (*Encoder, error) {
	if dataParts == 0 {
		return nil, errors.New("dataParts cannot be zero")
	}
	if totalParts < 2 {
		return nil, errors.New("totalParts cannot be less than 2")
	}

	var encoder = new(Encoder)
	encoder.total = totalParts
	encoder.data = dataParts
	encoder.resultQueue = make(chan []byte, 8*totalParts)
	var err error

	encoder.rs, err = reedsolomon.New(int(dataParts), int(totalParts)-int(dataParts))
	if err != nil {
		return nil, err
	}

	return encoder, nil
}

func (e *Encoder) encode(ctx context.Context, src io.Reader) {
	e.csn = 1
	defer func() { e.csn = 0 }()
	var size = PacketDataSize*e.data - ChunkHeaderSize
	var buf = make([]byte, size)
encode:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var n, err = src.Read(buf)
			if err != nil {
				fmt.Println(errors.Wrap(err, "src.Read(buf)"))
				continue encode
			}

			var chunk, _ = NewChunk(e.data, e.csn, 1, buf[:n])
			var data = chunk.Marshal(e.total)

			err = e.rs.Encode(data)
			if err != nil {
				fmt.Println(errors.Wrap(err, "e.rs.Encode(data)"))
				continue encode
			}

			e.csn++
			for i := range data {
				e.resultQueue <- data[i]
			}
		}
	}
}

func (e *Encoder) Encode(ctx context.Context, src io.Reader) (chan []byte, error) {
	if src == nil {
		return nil, errors.New("src is nil")
	}
	go e.encode(ctx, src)
	return e.resultQueue, nil
}
