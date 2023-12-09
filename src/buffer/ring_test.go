package buffer

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRing(t *testing.T) {
	var buf = make([]int, 4)
	var ring = NewRing(0, buf)
	var i int
	var ok bool

	ring.Write(false, 1)
	ring.Write(false, 2)
	ring.Write(false, 3)
	ring.Write(false, 4)

	var a0 = true
	go func() {
		ring.Write(false, 5)
		a0 = false
	}()
	time.Sleep(10 * time.Millisecond)
	require.True(t, a0)

	i, ok = ring.Read(true)
	require.True(t, ok)
	require.EqualValues(t, 1, i)
	time.Sleep(10 * time.Millisecond)
	require.False(t, a0)

	i, ok = ring.Read(true)
	require.True(t, ok)
	require.EqualValues(t, 2, i)

	i, ok = ring.Read(true)
	require.True(t, ok)
	require.EqualValues(t, 3, i)

	i, ok = ring.Read(true)
	require.True(t, ok)
	require.EqualValues(t, 4, i)

	i, ok = ring.Read(true)
	require.True(t, ok)
	require.EqualValues(t, 5, i)

	var a1 = true
	go func() {
		i, ok = ring.Read(true)
		require.True(t, ok)
		require.EqualValues(t, 6, i)
		a1 = false
	}()
	time.Sleep(10 * time.Millisecond)
	require.True(t, a1)

	ring.Write(false, 6)
	time.Sleep(10 * time.Millisecond)
	require.False(t, a1)
}
