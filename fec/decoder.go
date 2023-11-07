package fec

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/imakiri/fec/unit"
	"github.com/klauspost/reedsolomon"
	"io"
	"log"
	"math/rand"
	"sync/atomic"
)

func Decode(ctx context.Context, src io.ReadCloser, dst io.WriteCloser, data, parity int) error {
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
	src = unit.NewReader(src, PacketSize, true, false)
	dst = unit.NewWriter(dst, size, false, true)
	var buf = make([]byte, PacketSize)

	for seq := uint64(0); ; seq++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			err = decode(enc, src, dst, seq, buf)
			if err != nil {
				return err
			}
		}
	}
}

func decode(enc reedsolomon.Encoder, src io.ReadCloser, dst io.WriteCloser, order uint64, buf []byte) error {
	var n, err = src.Read(buf)
	if err != nil {
		return err
	}

	var id = rand.Uint32()
	var chunk = NewChunk(id, order, PacketDataSize, n, buf)
	var data = chunk.Marshal()
	var shards, _ = enc.Split(data)

	err = enc.Encode(shards)
	if err != nil {
		return err
	}

	var packet Packet
	for i := range shards {
		packet = NewPacket(uint16(i), id, shards[i])
		var data, _ = packet.Marshal()

		_, err = dst.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

type Decoder struct {
	c        Codec
	buf      []byte
	totalSrc *uint64
	totalDst *uint64
}

func (d Decoder) ChunkSize() int {
	return PacketSize
}

func (d *Decoder) read(ctx context.Context, rem *Packet, src io.Reader) ([]Packet, *Packet, error) {
	var packets = make([]Packet, 0, d.c.totalShards())
	var id uint16
	if rem != nil {
		id = rem.csn
		packets = append(packets, *rem)
	}

	for {
		if len(packets) == d.c.totalShards() {
			return packets, nil, nil
		}

		select {
		case <-ctx.Done():
			return packets, nil, nil
		default:
			var n, err = src.Read(d.buf)
			if err != nil {
				return packets, nil, err
			}

			atomic.AddUint64(d.totalSrc, uint64(n))

			var packet Packet
			var rem = packet.Unmarshal(d.buf)
			if rem != nil {
				return nil, nil, errors.New("not nil rem: invalid buffer size")
			}

			if !packet.Validate() {
				log.Println("invalid packet")
				continue
			}

			//log.Print("packet info ", packet.csn, packet.psn, packet.hash)

			if len(packets) == 0 {
				id = packet.csn
				packets = append(packets, packet)
			} else {
				if id == packet.csn {
					packets = append(packets, packet)
				} else {
					return packets, &packet, nil
				}
			}
		}
	}
}

func (d *Decoder) write(ctx context.Context, packets []Packet, dst io.Writer) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		var shards = make([][]byte, d.c.totalShards())
		var id *uint16
		for i := range packets {
			if packets[i].psn >= uint32(len(shards)) {
				continue
			}
			shards[packets[i].psn] = packets[i].data[:]
			if id == nil {
				id = &packets[i].csn
			}
		}
		if id == nil {
			id = new(uint16)
		}

		var err = d.c.enc.ReconstructData(shards)
		if err != nil {
			log.Println(err, " csn ", *id)
			return nil
		}

		for i := range shards[:d.c.dataShards] {
			select {
			case <-ctx.Done():
				return nil
			default:
				n, err := dst.Write(shards[i])
				if err != nil {
					return err
				}
				if n != PacketDataSize {
					return errors.New("n != PacketSize")
				}
				atomic.AddUint64(d.totalDst, uint64(n))
			}
		}

		return nil
	}
}

func (d *Decoder) Decode(ctx context.Context, src io.Reader, dst io.Writer) error {
	d.buf = make([]byte, d.ChunkSize())
	var rem *Packet
	var packets []Packet
	var err error
	for {
		packets, rem, err = d.read(ctx, rem, src)
		if err != nil {
			return err
		}

		err = d.write(ctx, packets, dst)
		if err != nil {
			return err
		}
	}
}
