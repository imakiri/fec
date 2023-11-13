package fec

import (
	"encoding/binary"
	"github.com/klauspost/reedsolomon"
)

const ChunkHeaderSize = 22

type Chunk struct {
	version uint8  // 0...1
	kind    uint8  // 1...2
	_       uint32 // 2...6
	csn     uint64 // 6...14
	size    uint64 // 14...22
	data    []byte
}

func NewChunk(csn uint64, data []byte) *Chunk {
	return &Chunk{
		version: 1,
		kind:    1,
		csn:     csn,
		size:    uint64(len(data)),
		data:    data,
	}
}

func (c *Chunk) Data() []byte {
	return c.data
}

func (c *Chunk) Parts() uint64 {
	return c.Len() / PacketSize
}

func (c *Chunk) Len() uint64 {
	var chunkSize = ChunkHeaderSize + uint64(len(c.data))
	var rem = chunkSize % PacketSize
	if rem == 0 {
		return chunkSize
	}
	return chunkSize + PacketSize - rem
}

func (c *Chunk) Marshal(shards int) ([][]byte, bool) {
	var b = reedsolomon.AllocAligned(shards, PacketDataSize)
	var i uint64 = 0

	b[i] = make([]byte, PacketDataSize)
	b[i][0] = c.version
	b[i][1] = c.kind
	//binary.LittleEndian.PutUint32(b[i][2:6], c.id)
	binary.LittleEndian.PutUint64(b[i][6:14], c.csn)
	binary.LittleEndian.PutUint64(b[i][14:22], uint64(len(c.data)))

	var copied = ChunkHeaderSize + copy(b[i][ChunkHeaderSize:], c.data)
	for i = 1; uint64(copied) < c.size; i++ {
		b[i] = make([]byte, PacketDataSize)
		copied += copy(b[i], c.data[copied:])
	}
	return b, uint64(copied) == c.size
}

func (c *Chunk) Unmarshal(data [][]byte) bool {
	var i uint64 = 0

	c.version = data[i][0]
	c.kind = data[i][1]
	//c.id = binary.LittleEndian.Uint32(data[i][2:6])
	c.csn = binary.LittleEndian.Uint64(data[i][6:14])
	c.size = binary.LittleEndian.Uint64(data[i][14:22])
	c.data = make([]byte, c.size)

	var copied = ChunkHeaderSize + copy(c.data, data[i][ChunkHeaderSize:])
	for i = 1; i < c.Parts(); i++ {
		copied += copy(c.data[copied:], data[i])
	}
	return c.size == uint64(copied)
}
