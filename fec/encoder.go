package fec

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/imakiri/fec/unit"
	"github.com/klauspost/reedsolomon"
	"io"
)

const Addr = 1

func Encode(ctx context.Context, src io.ReadCloser, dst io.WriteCloser, data, parity int) error {
	if src == nil {
		return errors.New("src is nil")
	}
	if dst == nil {
		return errors.New("dst is nil")
	}

	var enc, err = reedsolomon.New(data, parity)
	if err != nil {
		return errors.Wrap(err, "reedsolomon.New(dataShards, parityShards)")
	}

	var size = PacketDataSize*data - ChunkHeaderSize
	src = unit.NewReader(src, size, false, false)
	dst = unit.NewWriter(dst, PacketSize, true, true)
	var buf = make([]byte, size)

	for csn := uint64(0); ; csn++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			err = encode(enc, src, dst, data+parity, csn, buf)
			if err != nil {
				return err
			}
		}
	}
}

func encode(enc reedsolomon.Encoder, src io.ReadCloser, dst io.WriteCloser, shards int, csn uint64, buf []byte) error {
	var n, err = src.Read(buf)
	if err != nil {
		return err
	}

	var chunk = NewChunk(csn, buf[:n])
	var data, ok = chunk.Marshal(shards)
	if !ok {
		return errors.New("chunk.Marshal(shards) is not ok")
	}

	err = enc.Encode(data)
	if err != nil {
		return err
	}

	var packet *Packet
	for i := range data {
		packet = NewPacket(uint32(i), csn, Addr, data[i])
		var data, _ = packet.Marshal()

		_, err = dst.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}
