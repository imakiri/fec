package fec

import (
	"crypto/sha1"
	"encoding/binary"
)

const PacketSize = PacketHeaderSize + PacketDataSize + PacketHashSize
const PacketHeaderSize = 24
const PacketDataSize = 1024
const PacketHashSize = 8

type Packet struct {
	version uint8                // 0...1
	kind    uint8                // 1...2
	_       uint16               // 2...4
	psn     uint32               // 4...8  packet sequence number
	csn     uint64               // 8...16 chunk sequence number
	addr    uint64               // 16...24
	data    []byte               // 24...1048
	hash    [PacketHashSize]byte // 1048...1056
}

func NewPacket(psn uint32, csn uint64, addr uint64, data []byte) (*Packet, []byte) {
	var packet = &Packet{
		version: 2,
		kind:    1,
		psn:     psn,
		csn:     csn,
		addr:    addr,
		data:    make([]byte, PacketDataSize),
	}
	var copied = copy(packet.data, data)
	data = data[copied:]
	// The Good, the Bad and the Ugly
	if len(data) == 0 {
		return packet, nil
	}
	return packet, data
}

func hash(data []byte) [PacketHashSize]byte {
	var h = sha1.Sum(data)
	return [8]byte{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7]}
}

func (p *Packet) Data() []byte {
	if p == nil {
		return nil
	}
	return p.data
}

func (p *Packet) Marshal() []byte {
	var b = make([]byte, PacketSize)
	b[0] = p.version
	b[1] = p.kind
	binary.LittleEndian.PutUint32(b[4:8], p.psn)
	binary.LittleEndian.PutUint64(b[8:16], p.csn)
	binary.LittleEndian.PutUint64(b[16:24], p.addr)
	copy(b[PacketHeaderSize:PacketHeaderSize+PacketDataSize], p.data[:])
	var h = hash(b[:PacketHeaderSize+PacketDataSize])
	copy(b[PacketHeaderSize+PacketDataSize:PacketSize], h[:])
	return b
}

func (p *Packet) Unmarshal(data []byte) bool {
	if len(data) != PacketSize {
		return false
	}
	p.version = data[0]
	p.kind = data[1]
	p.psn = binary.LittleEndian.Uint32(data[4:8])
	p.csn = binary.LittleEndian.Uint64(data[8:16])
	p.addr = binary.LittleEndian.Uint64(data[16:24])
	p.data = make([]byte, PacketDataSize)
	copy(p.data[:], data[PacketHeaderSize:PacketHeaderSize+PacketDataSize])
	copy(p.hash[:], data[PacketHeaderSize+PacketDataSize:PacketSize])
	return p.hash == hash(data[:PacketHeaderSize+PacketDataSize])
}
