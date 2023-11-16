package fec

import (
	"github.com/go-faster/errors"
	"github.com/klauspost/reedsolomon"
)

type Codec struct {
	rs          reedsolomon.Encoder
	dataParts   uint64
	parityParts uint64
	totalParts  uint64

	csn uint64
}

func NewCodec(dataParts, parityParts uint64) (codec *Codec, err error) {
	codec = new(Codec)
	codec.rs, err = reedsolomon.New(int(dataParts), int(parityParts))
	if err != nil {
		return nil, errors.Wrap(err, "reedsolomon.New")
	}
	codec.dataParts = dataParts
	codec.parityParts = parityParts
	codec.totalParts = dataParts + parityParts
	return codec, nil
}

func (c *Codec) PartsData() uint64 {
	c.rs.Encode()
	return c.dataParts
}

func (c *Codec) PartsParity() uint64 {
	return c.dataParts
}

func (c *Codec) TotalParts() uint64 {
	return c.totalParts
}

func (c *Codec) MaxSize() uint64 {
	return PacketDataSize * c.totalParts
}

//func (c *Codec) Encode(data []byte, dst chan *Packet) error {
//	var chunk, err = NewChunk(c.dataParts, c.csn, 1, data)
//	if err != nil {
//		return err
//	}
//
//	var result = reedsolomon.AllocAligned(int(c.totalParts), PacketDataSize)
//	if !chunk.Marshal(result) {
//		return errors.New("!chunk.Marshal(result)")
//	}
//
//	err = c.rs.Encode(result)
//	if err != nil {
//		return errors.Wrap(err, "c.rs.Encode(result)")
//	}
//
//	return result, nil
//}

// Decode reconstruct data from
//func (c *Codec) Decode(data [][]byte) ([]byte, error) {
//	var parts = uint64(len(data))
//	if parts > c.totalParts {
//		return nil, errors.New("too many parts")
//	}
//	if parts < c.dataParts {
//		return nil, errors.New("not enough parts")
//	}
//
//	var err = c.rs.ReconstructData(data)
//	if err != nil {
//		return nil, errors.Wrap(err, "c.rs.ReconstructData(data)")
//	}
//
//	var chunk = new(Chunk)
//	chunk.Unmarshal(data[:c.dataParts])
//	return data[:c.dataParts], nil
//}
