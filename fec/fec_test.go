package fec

import (
	"bytes"
	"context"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"os"
	"os/signal"
	"sync"
	"testing"
)

func TestFEC(t *testing.T) {
	const dataParts = 8
	const totalParts = 12
	var err error
	var ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var expecting = bytes.NewBuffer(nil)
	var got = bytes.NewBuffer(nil)

	var encoder *Encoder
	var decoder *Decoder

	encoder, err = NewEncoder(dataParts, totalParts)
	assert.NoError(t, err)

	decoder, err = NewDecoder(dataParts, totalParts, 8, 8)
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
	const chunks = 2
	wg.Add(1)
	go func() {
		var size = encoder.IncomingSize()
		var buf = make([]byte, size)
		for i := 0; i < chunks; i++ {
			n, err := rand.Read(buf)
			assert.NoError(t, err)
			assert.EqualValues(t, size, n)

			n, err = expecting.Write(buf)
			assert.NoError(t, err)
			assert.EqualValues(t, size, n)

			encoderIn <- buf
		}
		//
		//encoderIn <- []byte("123")
		//expecting.Write([]byte("123"))
		//encoderIn <- []byte("456")
		//expecting.Write([]byte("456"))
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
	assert.EqualValues(t, expecting.Len(), got.Len())
	assert.EqualValues(t, expecting.Bytes(), got.Bytes())
}
