package observer

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testSrc struct {
	buf *bytes.Buffer
}

func (t testSrc) Read(p []byte) (n int, err error) {
	return t.buf.Read(p)
}

func (t testSrc) Close() error {
	t.buf.Reset()
	return nil
}

type testReporter struct {
	data [][2]uint64
}

func newTestReporter() *testReporter {
	return &testReporter{
		data: make([][2]uint64, 0, 10),
	}
}

func (t *testReporter) Report(total, delta uint64) {
	t.data = append(t.data, [2]uint64{total, delta})
}

func TestReader(t *testing.T) {
	var data = []byte("qwerty12345")
	var src = testSrc{
		buf: bytes.NewBuffer(data),
	}

	var rep = newTestReporter()
	var reader, err = NewReader(src, rep, time.Second)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	t.Log(rep.data)

	var buf0 = [][2]uint64{{0, 0}}
	require.Equal(t, buf0, rep.data)

	var buf = make([]byte, 3)
	var n int

	n, err = reader.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 3, n)

	time.Sleep(time.Second)
	t.Log(rep.data)

	var buf1 = [][2]uint64{{0, 0}, {3, 3}}
	require.Equal(t, buf1, rep.data)

	n, err = reader.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 3, n)

	time.Sleep(time.Second)
	t.Log(rep.data)

	var buf2 = [][2]uint64{{0, 0}, {3, 3}, {6, 3}}
	require.Equal(t, buf2, rep.data)
}
