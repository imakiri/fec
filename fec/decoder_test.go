package fec

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func assertPackets(t *testing.T, packets []*Packet, csn uint64, expect map[uint64]map[uint64][]byte) {
	var expectPackets, ok = expect[csn]
	if !ok {
		assert.Fail(t, "invalid test data")
		t.Fatal()
	}

	for i := range packets {
		var value, ok = expectPackets[uint64(i)]
		if ok && packets[i] == nil {
			assert.Fail(t, fmt.Sprintf("expect packet at csn: %d, psn: %d", csn, i), packets)
			t.Fatal()
		}
		if !ok && packets[i] != nil {
			assert.Fail(t, fmt.Sprintf("did not expect packet at csn: %d, psn: %d", csn, i), packets)
			t.Fatal()
		}
		if !ok && packets[i] == nil {
			continue
		}
		if ok && packets[i] != nil {
			assert.EqualValues(t, packets[i].csn, csn)
			for j := range value {
				assert.EqualValues(t, value[j], packets[i].data[j])
			}
			continue
		}
	}
}

func casePacket(csn, psn uint64, expect map[uint64]map[uint64][]byte) *Packet {
	var data = []byte(fmt.Sprintf("%d.%d", csn, psn))
	var packet, _ = NewPacket(uint32(psn), csn, KindData, data)
	if expect[csn] == nil {
		expect[csn] = make(map[uint64][]byte)
	}
	expect[csn][psn] = data
	return packet
}

func TestAssembler(t *testing.T) {
	var decoder, err = NewDecoder(4, 6, 3, 4)
	assert.NoError(t, err)

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	decoder.restoreQueue = make(chan []*Packet, 4)
	decoder.assemblyQueue = make(chan *Packet, 8)
	go decoder.assembly(ctx)

	var packets []*Packet
	var expect = make(map[uint64]map[uint64][]byte)

	decoder.assemblyQueue <- casePacket(1, 0, expect)
	decoder.assemblyQueue <- casePacket(1, 2, expect)
	decoder.assemblyQueue <- casePacket(1, 3, expect)
	decoder.assemblyQueue <- casePacket(1, 4, expect)

	packets = <-decoder.restoreQueue
	assertPackets(t, packets, 1, expect)

	decoder.assemblyQueue <- casePacket(2, 0, expect)
	decoder.assemblyQueue <- casePacket(3, 1, expect)
	decoder.assemblyQueue <- casePacket(2, 2, expect)
	decoder.assemblyQueue <- casePacket(3, 2, expect)
	decoder.assemblyQueue <- casePacket(2, 3, expect)
	decoder.assemblyQueue <- casePacket(3, 5, expect)
	decoder.assemblyQueue <- casePacket(2, 4, expect)
	decoder.assemblyQueue <- casePacket(3, 3, expect)

	packets = <-decoder.restoreQueue
	assertPackets(t, packets, 2, expect)

	packets = <-decoder.restoreQueue
	assertPackets(t, packets, 3, expect)

	decoder.assemblyQueue <- casePacket(4, 0, expect)
	decoder.assemblyQueue <- casePacket(5, 1, expect)
	decoder.assemblyQueue <- casePacket(6, 2, expect)
	decoder.assemblyQueue <- casePacket(7, 2, expect)
	decoder.assemblyQueue <- casePacket(8, 3, expect)
	decoder.assemblyQueue <- casePacket(9, 5, expect)
	decoder.assemblyQueue <- casePacket(9, 4, expect)
	decoder.assemblyQueue <- casePacket(9, 3, expect)
	decoder.assemblyQueue <- casePacket(9, 1, expect)

	packets = <-decoder.restoreQueue
	assertPackets(t, packets, 9, expect)
}

func TestDispatcher(t *testing.T) {
	var decoder, err = NewDecoder(8, 12, 8, 4)
	assert.NoError(t, err)

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	decoder.resultQueue = make(chan []byte, 8)
	decoder.dispatchQueue = make(chan *Chunk, 4)
	go decoder.dispatch(ctx)

	var chunk *Chunk
	var data []byte

	chunk, _ = NewChunk(8, 1, KindData, []byte(strconv.Itoa(0)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 2, KindData, []byte(strconv.Itoa(1)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 3, KindData, []byte(strconv.Itoa(2)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 4, KindData, []byte(strconv.Itoa(3)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 5, KindData, []byte(strconv.Itoa(4)))
	decoder.dispatchQueue <- chunk

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(0)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(1)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(2)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(3)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(4)), data)

	//

	chunk, _ = NewChunk(8, 6, KindData, []byte(strconv.Itoa(5)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 8, KindData, []byte(strconv.Itoa(7)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 10, KindData, []byte(strconv.Itoa(9)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 7, KindData, []byte(strconv.Itoa(6)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 9, KindData, []byte(strconv.Itoa(8)))
	decoder.dispatchQueue <- chunk

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(5)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(6)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(7)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(8)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(9)), data)

	//

	chunk, _ = NewChunk(8, 12, KindData, []byte(strconv.Itoa(11)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 13, KindData, []byte(strconv.Itoa(12)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 14, KindData, []byte(strconv.Itoa(13)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 15, KindData, []byte(strconv.Itoa(14)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 16, KindData, []byte(strconv.Itoa(15)))
	decoder.dispatchQueue <- chunk

	chunk, _ = NewChunk(8, 11, KindData, []byte(strconv.Itoa(10)))
	decoder.dispatchQueue <- chunk

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(11)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(12)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(13)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(14)), data)

	data = <-decoder.resultQueue
	assert.EqualValues(t, []byte(strconv.Itoa(15)), data)
}
