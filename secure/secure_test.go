package secure

import (
	"bytes"
	"crypto/rand"
	"github.com/imakiri/fec/codec"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSecure(t *testing.T) {
	var secret = make([]byte, 32)
	var n, err = rand.Reader.Read(secret)
	assert.NoError(t, err)
	assert.EqualValues(t, 32, n)

	var plainText = make([]byte, codec.PacketSize)
	copy(plainText, "1231234412312344qweqwetrt")
	var buf = make([]byte, 1472)
	var net = new(bytes.Buffer)

	reader, err := NewReader(codec.PacketSize, secret, net)
	assert.NoError(t, err)

	writer, err := NewWriter(codec.PacketSize, secret, net)
	assert.NoError(t, err)

	n, err = writer.Write(plainText)
	assert.NoError(t, err)

	n, err = reader.Read(buf)
	assert.NoError(t, err)

	assert.EqualValues(t, plainText[:25], buf[:25])

	n, err = writer.Write(plainText)
	assert.NoError(t, err)

	n, err = reader.Read(buf)
	assert.NoError(t, err)

	assert.EqualValues(t, plainText[:25], buf[:25])
}
