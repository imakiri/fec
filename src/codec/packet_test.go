package codec

import (
	"bytes"
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPacket(t *testing.T) {
	const size = 2 * PacketDataSize
	var data = make([]byte, size)
	n, _ := rand.Read(data)
	require.EqualValues(t, size, n)

	const psn = 23
	const csn = 3
	const addr = 1

	var packet, rem = NewPacket(1, csn, psn, addr, data)
	require.NotNil(t, rem)

	packet, rem = NewPacket(1, csn, psn, addr, data[:PacketDataSize:PacketDataSize])
	require.Nil(t, rem)
	var raw = packet.Marshal()

	packet = new(Packet)
	require.EqualValues(t, 0, packet.csn)

	var ok = packet.Unmarshal(raw)
	require.True(t, ok)
	require.EqualValues(t, psn, packet.psn)
	require.EqualValues(t, csn, packet.csn)
	require.EqualValues(t, addr, packet.addr)
	require.True(t, bytes.Equal(data[0:PacketDataSize], packet.data))
}
