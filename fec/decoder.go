package fec

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/imakiri/fec/unit"
	"github.com/klauspost/reedsolomon"
	"io"
	"log"
)

func Decode(ctx context.Context, src io.ReadCloser, dst io.WriteCloser, data, parity int) error {
	if src == nil {
		return errors.New("src is nil")
	}
	if dst == nil {
		return errors.New("dst is nil")
	}

	var total = uint32(data) + uint32(parity)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var enc, err = reedsolomon.New(data, parity)
	if err != nil {
		return errors.Wrap(err, "reedsolomon.New(dataShards, parityShards)")
	}

	var size = PacketDataSize*data - ChunkHeaderSize
	src = unit.NewReader(src, PacketSize, true, false)
	dst = unit.NewWriter(dst, size, false, true)
	var buf = make([]byte, PacketSize)

	queue, shutdown, err := NewQueue(uint32(data), total, 8)
	if err != nil {
		return errors.Wrap(err, "NewQueue")
	}

	var decodeErr = make(chan error)
	var processErr = make(chan error)
	go func() {
		decodeErr <- decode(ctx, src, queue, buf)
	}()
	go func() {
		processErr <- process(ctx, uint32(data), total, enc, dst, queue.processQueue)
	}()

	select {
	case err = <-decodeErr:
	case err = <-processErr:
	}
	cancel()
	shutdown()
	return err
}

func process(ctx context.Context, dataShards, shardsTotal uint32, enc reedsolomon.Encoder, dst io.WriteCloser, processQueue chan []*Packet) error {
	if dataShards > shardsTotal {
		return errors.New("invalid data shards number")
	}

	var packets []*Packet
	var shards = make([][]byte, shardsTotal)
	var chunk = new(Chunk)
	var err error
	for {
		select {
		case <-ctx.Done():
			return nil
		case packets = <-processQueue:
			for i := range packets {
				shards[packets[i].psn] = packets[i].data
			}

			err = enc.ReconstructData(shards)
			if err != nil {
				log.Println("enc.ReconstructData(shards): ", err)
				continue
			}

			var ok = chunk.Unmarshal(shards[:dataShards])
			if !ok {
				log.Println("chunk.Unmarshal(shards[:dataShards]): not ok")
				continue
			}

			n, err := dst.Write(chunk.Data())
			if err != nil {
				return err
			}
			if n != len(chunk.Data()) {
				return errors.New("incomplete write")
			}
		}
	}
}

func decode(ctx context.Context, src io.ReadCloser, queue *Queue, buf []byte) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var _, err = src.Read(buf)
			if err != nil {
				return err
			}

			var packet = new(Packet)
			var ok = packet.Unmarshal(buf)
			if !ok {
				continue
			}

			queue.Digest(packet)
		}
	}
}
