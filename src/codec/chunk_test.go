package codec

import (
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChunk(t *testing.T) {
	const size = 50 * PacketSize
	var data = make([]byte, size)
	n, _ := rand.Read(data)
	require.EqualValues(t, size, n)

	const csn = 3
	const kind = 1
	const parts = 8
	var chunk, rem = NewChunk(parts, csn, kind, data)
	require.NotNil(t, rem)

	chunk, rem = NewChunk(parts, csn, kind, data[:parts*PacketDataSize])
	require.Nil(t, rem)
	require.EqualValues(t, parts, chunk.parts)
	require.EqualValues(t, csn, chunk.csn)
	require.EqualValues(t, kind, chunk.kind)
	require.EqualValues(t, parts*PacketDataSize, chunk.size)
	require.EqualValues(t, data[:parts*PacketDataSize], chunk.data)

	chunk, rem = NewChunk(parts, csn, kind, data[:PacketDataSize])
	require.Nil(t, rem)
	require.EqualValues(t, parts, chunk.parts)
	require.EqualValues(t, csn, chunk.csn)
	require.EqualValues(t, kind, chunk.kind)
	require.EqualValues(t, PacketDataSize, chunk.size)
	require.EqualValues(t, parts, chunk.Parts())
	require.EqualValues(t, data[:PacketDataSize], chunk.data)

	const total = 10
	var payload = chunk.Marshal(total)
	require.EqualValues(t, total, len(payload))
	t.Log(payload)

	payload = payload[:parts] // Drop extra allocated parts

	chunk = new(Chunk)
	var ok = chunk.Unmarshal(payload)
	require.True(t, ok)

	require.EqualValues(t, parts, chunk.parts)
	require.EqualValues(t, csn, chunk.csn)
	require.EqualValues(t, kind, chunk.kind)
	require.EqualValues(t, PacketDataSize, chunk.size)
	require.EqualValues(t, parts, chunk.Parts())
	require.EqualValues(t, data[:PacketDataSize], chunk.data)
}
