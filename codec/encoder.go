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
	dispatcher struct {
		length uint64
		chunks uint64
		buf    []*Packet
	}

	aggregateQueue  chan []byte
	encodeQueue     chan []byte
	dispatcherQueue chan *Packet
	resultQueue     chan []byte
}

func NewEncoder(aggrtTimeout time.Duration, dispatcherSize, dataParts, totalParts uint64) (*Encoder, error) {
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

	encoder.dispatcher.chunks = dispatcherSize
	encoder.dispatcher.buf = make([]*Packet, totalParts*dispatcherSize)

	return encoder, nil
}

func (encoder *Encoder) close(ctx context.Context) {
	<-ctx.Done()
	close(encoder.aggregateQueue)
	close(encoder.dispatcherQueue)
	close(encoder.encodeQueue)
	close(encoder.resultQueue)
}

func (encoder *Encoder) IncomingSize() uint64 {
	return PacketDataSize*encoder.encoder.data - ChunkHeaderSize
}

func (encoder *Encoder) OutgoingSize() uint64 {
	return PacketSize
}

func (encoder *Encoder) aggregatorFlush() {
	var agData = make([]byte, len(encoder.aggregator.buf))
	copy(agData, encoder.aggregator.buf)
	encoder.aggregator.buf = encoder.aggregator.buf[0:0]
	select {
	case encoder.encodeQueue <- agData:
	}
}

func (encoder *Encoder) dispatch(ctx context.Context) {
	var packet *Packet
	var chunks = encoder.dispatcher.chunks
	var perChunk = encoder.encoder.total
	var size = chunks * perChunk
	//dispatch:
	for {
		select {
		case <-ctx.Done():
			return
		case packet = <-encoder.dispatcherQueue:
			var at = ((chunks * encoder.dispatcher.length) + (encoder.dispatcher.length / perChunk)) % size
			encoder.dispatcher.buf[at] = packet
			encoder.dispatcher.length++

			if encoder.dispatcher.length == size {
				for i := range encoder.dispatcher.buf {
					if encoder.dispatcher.buf[i] == nil {
						continue
					}
					select {
					case encoder.resultQueue <- encoder.dispatcher.buf[i].Marshal():
						encoder.dispatcher.buf[i] = nil
					}
				}
				encoder.dispatcher.length = 0
			}
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
				var kind uint8
				if encoder.encoder.csn == 1 {
					kind = 2
				} else {
					kind = 1
				}
				packet, rem = NewPacket(kind, encoder.encoder.csn, uint32(i), AddrFec, data[i])
				if rem != nil {
					log.Printf("encode: NewPacket: res is not nil: len %d", len(rem))
					continue
				}

				select {
				case encoder.dispatcherQueue <- packet:
				}
			}
			encoder.encoder.csn++
		}
	}
}

func (encoder *Encoder) aggregate(ctx context.Context) {
	var data []byte
	var sep int
	//aggregate:
	for {
		select {
		case <-ctx.Done():
			return
		case <-encoder.aggregator.timer.C:
			encoder.aggregatorFlush()
		case data = <-encoder.aggregateQueue:
			for len(data) > 0 {
				sep = min(cap(encoder.aggregator.buf)-len(encoder.aggregator.buf), len(data))
				encoder.aggregator.buf = append(encoder.aggregator.buf, data[:sep]...)
				data = data[sep:]
				if len(encoder.aggregator.buf) == cap(encoder.aggregator.buf) {
					encoder.aggregatorFlush()
				}
			}
			encoder.aggregator.timer.Reset(encoder.aggregator.timeout)
		}
	}
}

func (encoder *Encoder) Encode(ctx context.Context) (in, out chan []byte, err error) {
	encoder.resultQueue = make(chan []byte, 8*encoder.dispatcher.chunks)
	encoder.dispatcherQueue = make(chan *Packet, 8*encoder.dispatcher.chunks)
	encoder.encodeQueue = make(chan []byte, 64*encoder.encoder.data)
	encoder.aggregateQueue = make(chan []byte, 64*encoder.encoder.data)

	go encoder.dispatch(ctx)
	go encoder.aggregate(ctx)
	go encoder.encode(ctx)
	go encoder.close(ctx)
	return encoder.aggregateQueue, encoder.resultQueue, nil
}
