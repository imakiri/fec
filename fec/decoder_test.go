package fec

import (
	"context"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

//func TestAssembler(t *testing.T) {
//	var decoder, err = NewDecoder(8, 12, 8, 4)
//	assert.NoError(t, err)
//
//	var ctx, cancel = context.WithCancel(context.Background())
//	defer cancel()
//	go decoder.restore(ctx)
//
//	var packet *Packet
//	var packets []*Packet
//
//	packet, _ = NewPacket(0, 0, 1, []byte(strconv.Itoa(10)))
//	decoder.assemblyQueue <- packet
//
//	packets = <-decoder.restoreQueue
//
//}

func TestDispatcher(t *testing.T) {
	var decoder, err = NewDecoder(8, 12, 8, 4)
	assert.NoError(t, err)

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	go decoder.dispatch(ctx)

	var chunk *Chunk
	var data []byte

	chunk, _ = NewChunk(8, 0, 1, []byte(strconv.Itoa(0)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 1, 1, []byte(strconv.Itoa(1)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 2, 1, []byte(strconv.Itoa(2)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 3, 1, []byte(strconv.Itoa(3)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 4, 1, []byte(strconv.Itoa(4)))
	decoder.dispatchQueue <- chunk

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(0)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(1)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(2)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(3)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(4)), data)

	//

	chunk, _ = NewChunk(8, 5, 1, []byte(strconv.Itoa(5)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 7, 1, []byte(strconv.Itoa(7)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 9, 1, []byte(strconv.Itoa(9)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 6, 1, []byte(strconv.Itoa(6)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 8, 1, []byte(strconv.Itoa(8)))
	decoder.dispatchQueue <- chunk

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(5)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(6)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(7)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(8)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(9)), data)

	//

	chunk, _ = NewChunk(8, 11, 1, []byte(strconv.Itoa(11)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 12, 1, []byte(strconv.Itoa(12)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 13, 1, []byte(strconv.Itoa(13)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 14, 1, []byte(strconv.Itoa(14)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 15, 1, []byte(strconv.Itoa(15)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 10, 1, []byte(strconv.Itoa(10)))
	decoder.dispatchQueue <- chunk

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(11)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(12)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(13)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(14)), data)

	data = <-decoder.returnQueue
	assert.EqualValues(t, []byte(strconv.Itoa(15)), data)
}
