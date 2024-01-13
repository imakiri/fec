package codec

import (
	"bytes"
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPacket(t *testing.T) {
	const size = 2 * PacketV2DataSize
	var data = make([]byte, size)
	n, _ := rand.Read(data)
	require.EqualValues(t, size, n)

	const psn = 23
	const csn = 3
	const addr = 1

	var packet, rem = NewPacketV2(1, csn, psn, addr, data)
	require.NotNil(t, rem)

	packet, rem = NewPacketV2(1, csn, psn, addr, data[:PacketV2DataSize:PacketV2DataSize])
	require.Nil(t, rem)
	var raw = packet.Marshal()

	packet = new(PacketV2)
	require.EqualValues(t, 0, packet.csn)

	var ok = packet.Unmarshal(raw)
	require.True(t, ok)
	require.EqualValues(t, psn, packet.psn)
	require.EqualValues(t, csn, packet.csn)
	require.EqualValues(t, addr, packet.addr)
	require.True(t, bytes.Equal(data[0:PacketV2DataSize], packet.data))
}
