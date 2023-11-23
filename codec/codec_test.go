package codec

import (
	"bytes"
	"context"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"os"
	"os/signal"
	"sync"
	"testing"
	"time"
)

func TestCodec(t *testing.T) {
	const dataParts = 4
	const totalParts = 8
	var err error
	var ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var expecting = bytes.NewBuffer(nil)
	var got = bytes.NewBuffer(nil)

	var encoder *Encoder
	var decoder *Decoder

	encoder, err = NewEncoder(20*time.Millisecond, 32, dataParts, totalParts)
	assert.NoError(t, err)

	decoder, err = NewDecoder(dataParts, totalParts, 128, 16)
	assert.NoError(t, err)

	encoderIn, encoderOut, err := encoder.Encode(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, encoderIn)
	assert.NotNil(t, encoderOut)

	decoderIn, decoderOut, err := decoder.Decode(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, decoderIn)
	assert.NotNil(t, decoderOut)

	var wg = new(sync.WaitGroup)
	const chunks = 32
	wg.Add(1)
	go func() {
		var size = encoder.IncomingSize() - 3
		for i := 0; i < chunks; i++ {
			var buf = make([]byte, size)

			n, err := rand.Read(buf)
			assert.NoError(t, err)
			assert.EqualValues(t, size, n)

			n, err = expecting.Write(buf)
			assert.NoError(t, err)
			assert.EqualValues(t, size, n)

			encoderIn <- buf
		}

		//var data = make([]byte, encoder.IncomingSize())
		//for i := range data {
		//	data[i] = 1
		//}
		//encoderIn <- data
		//expecting.Write(data)
		//
		//data = make([]byte, encoder.IncomingSize())
		//for i := range data {
		//	data[i] = 2
		//}
		//encoderIn <- data
		//expecting.Write(data)
	}()

	go func() {
		var data []byte
		var packet *Packet
		var ok bool
		for {
			select {
			case <-ctx.Done():
				return
			case data = <-encoderOut:
				// no packet loss for now
				packet = new(Packet)
				ok = packet.Unmarshal(data)
				assert.True(t, ok)
				//t.Log(packet)
				select {
				case decoderIn <- data:
					continue
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	go func() {
		var data []byte
		for i := 0; i < chunks; i++ {
			select {
			case <-ctx.Done():
				return
			case data = <-decoderOut:
				var n, err = got.Write(data)
				assert.NoError(t, err)
				assert.EqualValues(t, len(data), n)
			}
		}
		wg.Done()
	}()

	wg.Wait()
	t.Log(got.Bytes()[20])
	t.Log(expecting.Bytes()[20])

	assert.EqualValues(t, expecting.Len(), got.Len())
	assert.EqualValues(t, expecting.Bytes(), got.Bytes())
}
