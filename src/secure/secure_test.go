package secure

import (
	"bytes"
	"crypto/rand"
	"github.com/imakiri/stream/src/codec"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSecure(t *testing.T) {
	var secret = make([]byte, 32)
	var n, err = rand.Reader.Read(secret)
	require.NoError(t, err)
	require.EqualValues(t, 32, n)

	var plainText = make([]byte, codec.PacketSize)
	copy(plainText, "1231234412312344qweqwetrt")
	var buf = make([]byte, 1472)
	var net = new(bytes.Buffer)

	reader, err := NewReader(codec.PacketSize, secret, net)
	require.NoError(t, err)

	writer, err := NewWriter(codec.PacketSize, secret, net)
	require.NoError(t, err)

	n, err = writer.Write(plainText)
	require.NoError(t, err)

	n, err = reader.Read(buf)
	require.NoError(t, err)

	require.EqualValues(t, plainText[:25], buf[:25])

	n, err = writer.Write(plainText)
	require.NoError(t, err)

	n, err = reader.Read(buf)
	require.NoError(t, err)

	require.EqualValues(t, plainText[:25], buf[:25])
}
