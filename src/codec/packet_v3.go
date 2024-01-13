package codec

import (
	"encoding/binary"
)

const PacketV3Size = PacketV3HeaderSize + PacketV3DataSize + PacketV3HashSize
const PacketV3HeaderSize = 24
const PacketV3DataSize = 1024
const PacketV3HashSize = 8

type PacketV3 struct {
	version uint8                  // 0...1
	kind    uint8                  // 1...2
	_       uint16                 // 2...4
	psn     uint32                 // 4...8  packet sequence number
	csn     uint64                 // 8...16 chunk sequence number
	addr    uint64                 // 16...24
	data    []byte                 // 24...1048
	hash    [PacketV3HashSize]byte // 1048...1056
}

func NewPacketV3(kind uint8, csn uint64, psn uint32, addr uint64, data []byte) (*PacketV3, []byte) {
	var packet = &PacketV3{
		version: 3,
		kind:    kind,
		psn:     psn,
		csn:     csn,
		addr:    addr,
		data:    make([]byte, PacketV3DataSize),
	}
	var copied = copy(packet.data, data)
	data = data[copied:]
	// The Good, the Bad and the Ugly
	if len(data) == 0 {
		return packet, nil
	}
	return packet, data
}

func (p *PacketV3) Data() []byte {
	if p == nil {
		return nil
	}
	return p.data
}

func (p *PacketV3) Marshal() []byte {
	var b = make([]byte, PacketV3Size)
	b[0] = p.version
	b[1] = p.kind
	binary.LittleEndian.PutUint32(b[4:8], p.psn)
	binary.LittleEndian.PutUint64(b[8:16], p.csn)
	binary.LittleEndian.PutUint64(b[16:24], p.addr)
	copy(b[PacketV3HeaderSize:PacketV3HeaderSize+PacketV3DataSize], p.data[:])
	var h = hash(b[:PacketV3HeaderSize+PacketV3DataSize])
	copy(b[PacketV3HeaderSize+PacketV3DataSize:PacketV3Size], h[:])
	return b
}

func (p *PacketV3) Unmarshal(data []byte) bool {
	if len(data) != PacketV3Size {
		return false
	}
	p.version = data[0]
	p.kind = data[1]
	p.psn = binary.LittleEndian.Uint32(data[4:8])
	p.csn = binary.LittleEndian.Uint64(data[8:16])
	p.addr = binary.LittleEndian.Uint64(data[16:24])
	p.data = make([]byte, PacketV3DataSize)
	copy(p.data[:], data[PacketV3HeaderSize:PacketV3HeaderSize+PacketV3DataSize])
	copy(p.hash[:], data[PacketV3HeaderSize+PacketV3DataSize:PacketV3Size])
	return p.hash == hash(data[:PacketV3HeaderSize+PacketV3DataSize])
}
