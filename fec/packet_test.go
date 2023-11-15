package fec

import (
	"bytes"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPacket(t *testing.T) {
	const size = 2 * PacketDataSize
	var data = make([]byte, size)
	n, _ := rand.Read(data)
	assert.EqualValues(t, size, n)

	const psn = 23
	const csn = 3
	const addr = 1

	var packet, rem = NewPacket(psn, csn, addr, data)
	assert.NotNil(t, rem)

	packet, rem = NewPacket(psn, csn, addr, data[:PacketDataSize:PacketDataSize])
	assert.Nil(t, rem)
	var raw, eq = packet.Marshal()
	assert.False(t, eq)

	packet = new(Packet)
	assert.EqualValues(t, 0, packet.csn)

	packet.Unmarshal(raw)
	assert.EqualValues(t, psn, packet.psn)
	assert.EqualValues(t, csn, packet.csn)
	assert.EqualValues(t, addr, packet.addr)
	assert.True(t, bytes.Equal(data[0:PacketDataSize], packet.data))
}
