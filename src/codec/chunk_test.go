package codec

import (
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestChunk(t *testing.T) {
	const size = 50 * PacketSize
	var data = make([]byte, size)
	n, _ := rand.Read(data)
	assert.EqualValues(t, size, n)

	const csn = 3
	const kind = 1
	const parts = 8
	var chunk, rem = NewChunk(parts, csn, kind, data)
	assert.NotNil(t, rem)

	chunk, rem = NewChunk(parts, csn, kind, data[:parts*PacketDataSize])
	assert.Nil(t, rem)
	assert.EqualValues(t, parts, chunk.parts)
	assert.EqualValues(t, csn, chunk.csn)
	assert.EqualValues(t, kind, chunk.kind)
	assert.EqualValues(t, parts*PacketDataSize, chunk.size)
	assert.EqualValues(t, data[:parts*PacketDataSize], chunk.data)

	chunk, rem = NewChunk(parts, csn, kind, data[:PacketDataSize])
	assert.Nil(t, rem)
	assert.EqualValues(t, parts, chunk.parts)
	assert.EqualValues(t, csn, chunk.csn)
	assert.EqualValues(t, kind, chunk.kind)
	assert.EqualValues(t, PacketDataSize, chunk.size)
	assert.EqualValues(t, parts, chunk.Parts())
	assert.EqualValues(t, data[:PacketDataSize], chunk.data)

	const total = 10
	var payload = chunk.Marshal(total)
	assert.EqualValues(t, total, len(payload))
	t.Log(payload)

	payload = payload[:parts] // Drop extra allocated parts

	chunk = new(Chunk)
	var ok = chunk.Unmarshal(payload)
	assert.True(t, ok)

	assert.EqualValues(t, parts, chunk.parts)
	assert.EqualValues(t, csn, chunk.csn)
	assert.EqualValues(t, kind, chunk.kind)
	assert.EqualValues(t, PacketDataSize, chunk.size)
	assert.EqualValues(t, parts, chunk.Parts())
	assert.EqualValues(t, data[:PacketDataSize], chunk.data)
}
