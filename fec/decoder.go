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
		size      uint32
		last      uint64
		first     uint64
		lastIndex int
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

func NewDecoder(dataParts, totalParts, assemblerLines, dispatcherSize uint32) (decoder *Decoder, err error) {
	decoder = new(Decoder)
	decoder.dispatcher.size = dispatcherSize
	decoder.dispatcher.chunks = make([]*Chunk, dispatcherSize)
	decoder.dispatcher.csn = make([]uint64, dispatcherSize)

	decoder.restorer.rs, err = reedsolomon.New(int(dataParts), int(totalParts)-int(dataParts))
	if err != nil {
		return nil, err
	}
	decoder.restorer.data = dataParts
	decoder.restorer.total = totalParts

	decoder.assembler.order = make([]uint32, assemblerLines)
	decoder.assembler.csns = make([]uint64, assemblerLines)
	decoder.assembler.chunks = make([][]*Packet, assemblerLines)
	for i := range decoder.assembler.chunks {
		decoder.assembler.chunks[i] = make([]*Packet, totalParts)
	}

	decoder.dispatchQueue = make(chan *Chunk, 4)
	decoder.restoreQueue = make(chan []*Packet, 4)
	decoder.assemblyQueue = make(chan *Packet, 8)
	decoder.returnQueue = make(chan []byte, 8)

	return decoder, nil
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

			if chunk.csn <= decoder.dispatcher.last && chunk.csn != 0 {
				log.Println("detached: csn: ", chunk.csn)
				continue dispatch
			}

			decoder.dispatcher.first = max(decoder.dispatcher.first, chunk.csn)

			if chunk.csn != decoder.dispatcher.awaiting {
				var at = chunk.csn % uint64(decoder.dispatcher.size)
				if decoder.dispatcher.chunks[at] != nil {
					log.Println("lost: csn: ", decoder.dispatcher.awaiting)
					decoder.returnQueue <- decoder.dispatcher.chunks[at].Data()
					decoder.dispatcher.last = decoder.dispatcher.csn[at]
					decoder.dispatcher.awaiting = decoder.dispatcher.csn[at] + 1

					decoder.dispatcher.csn[at] = chunk.csn
					decoder.dispatcher.chunks[at] = chunk
				} else {
					decoder.dispatcher.csn[at] = chunk.csn
					decoder.dispatcher.chunks[at] = chunk
					continue dispatch
				}
			} else {
				decoder.returnQueue <- chunk.Data()
				decoder.dispatcher.last = chunk.csn
				decoder.dispatcher.awaiting++
			}

			var length = decoder.dispatcher.first - decoder.dispatcher.last
			if length == 0 {
				continue dispatch
			}

			for i := decoder.dispatcher.last; i < decoder.dispatcher.first; i++ {
				var at = (i + 1) % uint64(decoder.dispatcher.size)
				if decoder.dispatcher.chunks[at] == nil {
					continue dispatch
				}
				if decoder.dispatcher.csn[at] != decoder.dispatcher.awaiting {
					continue dispatch
				}

				decoder.returnQueue <- decoder.dispatcher.chunks[at].Data()
				decoder.dispatcher.last = decoder.dispatcher.chunks[at].csn
				decoder.dispatcher.awaiting++
				decoder.dispatcher.chunks[at] = nil
			}
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

			if packet.csn <= decoder.assembler.last {
				log.Println("dropping invalid packet: outdated csn: ", packet.csn)
				continue assembly
			}

			if packet.csn > decoder.assembler.first {
				// pushing new chunk
				for i := uint32(0); i < decoder.assembler.size; i++ {
					if decoder.assembler.order[i] == 0 {
						log.Println("dropping outdated chunk: csn: ", decoder.assembler.csns[i])
						decoder.assembler.last = decoder.assembler.csns[i]
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
				decoder.assembler.first = packet.csn
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
			decoder.assembler.last = packet.csn
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
