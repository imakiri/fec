package fec

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/klauspost/reedsolomon"
	"io"
	"log"
)

type Decoder struct {
	assembler struct {
		size       uint32
		lastPopped uint64
		lastPushed uint64
		lastIndex  int
		// inlined []element
		order  []uint32
		csns   []uint64
		chunks [][]*Packet
	}
	restorer struct {
		rs    reedsolomon.Encoder
		total uint32
		data  uint32
	}
	dispatcher struct {
		size     uint32
		len      uint32
		first    uint64
		last     uint64
		awaiting uint64
		csn      []uint64
		chunks   []*Chunk
	}

	assemblyQueue chan *Packet
	restoreQueue  chan []*Packet
	dispatchQueue chan *Chunk
	returnQueue   chan []byte
}

func NewDigester(dataParts, totalParts, size uint32) (queue *Decoder, shutdown func(), err error) {
	queue = new(Decoder)
	queue.size = size
	queue.restorer.data = dataParts
	queue.restorer.total = totalParts
	queue.assembler.order = make([]uint32, size)
	queue.assembler.csns = make([]uint64, size)
	queue.assembler.chunks = make([][]*Packet, size)

	queue.dispatchQueue = make(chan *Chunk, size)
	queue.restoreQueue = make(chan []*Packet, size)
	queue.assemblyQueue = make(chan *Packet, size)
	for i := range queue.assembler.chunks {
		queue.assembler.chunks[i] = make([]*Packet, totalParts)
	}
	shutdown = func() {
		close(queue.dispatchQueue)
	}

	return queue, shutdown, nil
}

func (decoder *Decoder) dispatch(ctx context.Context) {
	var chunk *Chunk
dispatch:
	for {
		select {
		case <-ctx.Done():
			return
		case chunk = <-decoder.dispatchQueue:
			if chunk == nil {
				log.Println("dropping invalid chunk: nil chunk")
				continue dispatch
			}

			if chunk.csn != decoder.dispatcher.awaiting {
				// if there is still space left for new chunks
				if decoder.dispatcher.len < decoder.dispatcher.size {
					decoder.dispatcher.chunks[decoder.dispatcher.len] = chunk
					decoder.dispatcher.csn[decoder.dispatcher.len] = chunk.csn
					decoder.dispatcher.len++
				} else {

				}
				continue dispatch
			}

			decoder.returnQueue <- chunk.Data()

		}
	}
}

func (decoder *Decoder) restore(ctx context.Context) {
	var packets []*Packet
	var data = make([][]byte, decoder.restorer.total)
	var chunk *Chunk
	var err error
	var ok bool
restore:
	for {
		select {
		case <-ctx.Done():
			return
		case packets = <-decoder.restoreQueue:
			for i := range data {
				data[i] = nil
			}
			for i := range data {
				if packets[i].psn > decoder.restorer.total {
					log.Println("dropping invalid packet: invalid psn: ", packets[i].psn)
					continue restore
				}
				data[packets[i].psn] = packets[i].data
			}

			err = decoder.restorer.rs.ReconstructData(data)
			if err != nil {
				log.Println(errors.Wrap(err, "ReconstructData(data)"))
				continue restore
			}

			ok = chunk.Unmarshal(data[:decoder.restorer.data])
			if !ok {
				log.Println("dropping invalid packet: chunk.Unmarshal: not ok")
				continue restore
			}

			decoder.dispatchQueue <- chunk
		}
	}
}

func (decoder *Decoder) assembly(ctx context.Context) {
	var packet *Packet
assembly:
	for {
		select {
		case <-ctx.Done():
			return
		case packet = <-decoder.assemblyQueue:
			if packet == nil {
				log.Println("dropping invalid packet: packet is nil")
				continue assembly
			}

			if packet.csn <= decoder.assembler.lastPopped {
				log.Println("dropping invalid packet: outdated csn: ", packet.csn)
				continue assembly
			}

			if packet.csn > decoder.assembler.lastPushed {
				// pushing new chunk
				for i := uint32(0); i < decoder.assembler.size; i++ {
					if decoder.assembler.order[i] == 0 {
						log.Println("dropping outdated chunk: csn: ", decoder.assembler.csns[i])
						decoder.assembler.lastPopped = decoder.assembler.csns[i]
						// dropping the oldest chunk and replacing it with a new one, containing single packet
						decoder.assembler.order[i] = decoder.assembler.size - 1
						decoder.assembler.csns[i] = packet.csn
						decoder.assembler.chunks[i] = decoder.assembler.chunks[i][0:1]
						// pushing new packet
						decoder.assembler.chunks[i][0] = packet
					} else {
						// due to the push we shift the order of all other packets by -1
						decoder.assembler.order[i]--
					}
				}
				decoder.assembler.lastPushed = packet.csn
				continue assembly
			}

			var index uint32
			for i := uint32(0); i < decoder.assembler.size; i++ {
				// looking for an index related to the incoming packet at which its chunk is stored
				if decoder.assembler.csns[i] == packet.csn {
					decoder.assembler.chunks[i] = append(decoder.assembler.chunks[i], packet)
					if uint32(len(decoder.assembler.chunks[i])) == decoder.restorer.data {
						index = i
						break // breaking the loop when we found a complete chunk
					}
					continue assembly
				}
			}

			decoder.restoreQueue <- decoder.assembler.chunks[index]

			for i := uint32(0); i < decoder.assembler.size; i++ {
				if i == index || decoder.assembler.order[i] > decoder.assembler.order[index] || decoder.assembler.csns[i] == 0 {
					continue
				}
				decoder.assembler.order[i]++
			}

			decoder.assembler.csns[index] = 0
			decoder.assembler.order[index] = 0
			decoder.assembler.chunks[index] = decoder.assembler.chunks[index][0:0]
			decoder.assembler.lastPopped = packet.csn
		}
	}
}

func (decoder *Decoder) decode(ctx context.Context, src io.Reader) {
	var n int
	var err error
	var data = make([]byte, PacketSize)
	var packet *Packet
	var ok bool
decode:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err = src.Read(data)
			if err != nil {
				log.Println(errors.Wrap(err, "invalid read"))
				continue decode
			}
			if n != PacketSize {
				log.Println("invalid read: read only ", n, " bytes")
				continue decode
			}

			packet = new(Packet)
			ok = packet.Unmarshal(data)
			if !ok {
				log.Println("packet.Unmarshal(data) is not ok")
				continue decode
			}

			decoder.assemblyQueue <- packet
		}
	}
}

func (decoder *Decoder) Decode(ctx context.Context, src io.Reader) (cancel func()) {
	if src == nil {
		return func() {}
	}
	ctx, cancel = context.WithCancel(ctx)
	go decoder.dispatch(ctx)
	go decoder.restore(ctx)
	go decoder.assembly(ctx)
	go decoder.decode(ctx, src)
	return cancel
}
