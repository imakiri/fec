package codec

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/klauspost/reedsolomon"
	"log"
	"time"
)

type Encoder struct {
	aggregator struct {
		timeout time.Duration
		timer   *time.Timer
		buf     []byte
	}
	encoder struct {
		rs    reedsolomon.Encoder
		total uint64
		data  uint64
		csn   uint64
	}

	aggregateQueue chan []byte
	encodeQueue    chan []byte
	resultQueue    chan []byte
}

func NewEncoder(aggrtTimeout time.Duration, dataParts, totalParts uint64) (*Encoder, error) {
	if dataParts == 0 {
		return nil, errors.New("dataParts cannot be zero")
	}
	if totalParts < 2 {
		return nil, errors.New("totalParts cannot be less than 2")
	}

	var encoder = new(Encoder)
	encoder.encoder.total = totalParts
	encoder.encoder.data = dataParts

	var err error
	encoder.encoder.rs, err = reedsolomon.New(int(dataParts), int(totalParts)-int(dataParts))
	if err != nil {
		return nil, err
	}

	encoder.aggregator.buf = make([]byte, 0, encoder.IncomingSize())
	encoder.aggregator.timeout = aggrtTimeout
	encoder.aggregator.timer = time.NewTimer(100000000 * time.Second)

	return encoder, nil
}

func (encoder *Encoder) close(ctx context.Context) {
	<-ctx.Done()
	close(encoder.aggregateQueue)
	close(encoder.resultQueue)
}

func (encoder *Encoder) IncomingSize() uint64 {
	return PacketDataSize*encoder.encoder.data - ChunkHeaderSize
}

func (encoder *Encoder) OutgoingSize() uint64 {
	return PacketSize
}

func (encoder *Encoder) flush() {
	var agData = make([]byte, len(encoder.aggregator.buf))
	copy(agData, encoder.aggregator.buf)
	encoder.aggregator.buf = encoder.aggregator.buf[0:0]
	encoder.encodeQueue <- agData
}

func (encoder *Encoder) aggregate(ctx context.Context) {
	var data []byte
aggregate:
	for {
		select {
		case <-ctx.Done():
			return
		case <-encoder.aggregator.timer.C:
			encoder.flush()
		case data = <-encoder.aggregateQueue:
			var newLength = len(data) + len(encoder.aggregator.buf)
			if newLength < cap(encoder.aggregator.buf) {
				encoder.aggregator.buf = append(encoder.aggregator.buf, data...)
				continue aggregate
			}

			var c = newLength / cap(encoder.aggregator.buf)
			var sep int
			for i := 0; i < c; i++ {
				sep += cap(encoder.aggregator.buf)
				encoder.aggregator.buf = append(encoder.aggregator.buf, data[:sep]...)
				encoder.flush()
				data = data[sep:]
			}

			var rem = newLength % cap(encoder.aggregator.buf)
			encoder.aggregator.buf = append(encoder.aggregator.buf, data[rem:]...)
			encoder.aggregator.timer.Reset(encoder.aggregator.timeout)
		}
	}
}

func (encoder *Encoder) encode(ctx context.Context) {
	encoder.encoder.csn = 1
	defer func() { encoder.encoder.csn = 0 }()
	var err error
	var data []byte
	var packet *Packet
encode:
	for {
		select {
		case <-ctx.Done():
			return
		case data = <-encoder.encodeQueue:
			var chunk, rem = NewChunk(encoder.encoder.data, encoder.encoder.csn, KindData, data)
			if rem != nil {
				log.Printf("encode: NewChunk: res is not nil: len %d", len(rem))
				continue
			}
			var data = chunk.Marshal(encoder.encoder.total)

			err = encoder.encoder.rs.Encode(data)
			if err != nil {
				fmt.Println(errors.Wrap(err, "encode: rs.Encode"))
				continue encode
			}

			for i := range data {
				packet, rem = NewPacket(encoder.encoder.csn, uint32(i), AddrFec, data[i])
				if rem != nil {
					log.Printf("encode: NewPacket: res is not nil: len %d", len(rem))
					continue
				}
				encoder.resultQueue <- packet.Marshal()
			}
			encoder.encoder.csn++
		}
	}
}

func (encoder *Encoder) Encode(ctx context.Context) (in, out chan []byte, err error) {
	encoder.resultQueue = make(chan []byte, 8*encoder.encoder.data)
	encoder.encodeQueue = make(chan []byte, 8*encoder.encoder.data)
	encoder.aggregateQueue = make(chan []byte, 8*encoder.encoder.data)

	go encoder.aggregate(ctx)
	go encoder.encode(ctx)
	go encoder.close(ctx)
	return encoder.aggregateQueue, encoder.resultQueue, nil
}
