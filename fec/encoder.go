package fec

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/klauspost/reedsolomon"
	"log"
)

type Encoder struct {
	rs    reedsolomon.Encoder
	total uint64
	data  uint64
	csn   uint64

	argumentQueue chan []byte
	resultQueue   chan []byte
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

	var err error
	encoder.rs, err = reedsolomon.New(int(dataParts), int(totalParts)-int(dataParts))
	if err != nil {
		return nil, err
	}

	return encoder, nil
}

func (encoder *Encoder) close(ctx context.Context) {
	<-ctx.Done()
	close(encoder.argumentQueue)
	close(encoder.resultQueue)
}

func (encoder *Encoder) IncomingSize() uint64 {
	return PacketDataSize*encoder.data - ChunkHeaderSize
}

func (encoder *Encoder) OutgoingSize() uint64 {
	return PacketSize
}

func (encoder *Encoder) encode(ctx context.Context) {
	encoder.csn = 1
	defer func() { encoder.csn = 0 }()
	var err error
	var data []byte
	var packet *Packet
encode:
	for {
		select {
		case <-ctx.Done():
			return
		case data = <-encoder.argumentQueue:
			var chunk, rem = NewChunk(encoder.data, encoder.csn, KindData, data)
			if rem != nil {
				log.Printf("encode: NewChunk: res is not nil: len %d", len(rem))
				continue
			}
			var data = chunk.Marshal(encoder.total)

			err = encoder.rs.Encode(data)
			if err != nil {
				fmt.Println(errors.Wrap(err, "encode: rs.Encode"))
				continue encode
			}

			for i := range data {
				packet, rem = NewPacket(encoder.csn, uint32(i), AddrFec, data[i])
				if rem != nil {
					log.Printf("encode: NewPacket: res is not nil: len %d", len(rem))
					continue
				}
				encoder.resultQueue <- packet.Marshal()
			}
			encoder.csn++
		}
	}
}

func (encoder *Encoder) Encode(ctx context.Context) (in, out chan []byte, err error) {
	encoder.resultQueue = make(chan []byte, 8*encoder.data)
	encoder.argumentQueue = make(chan []byte, 8*encoder.data)

	go encoder.encode(ctx)
	go encoder.close(ctx)
	return encoder.argumentQueue, encoder.resultQueue, nil
}
