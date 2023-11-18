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
		size  uint64
		lines uint64
		last  uint64
		first uint64
		// inlined []element
		csn     []uint64
		found   []uint64
		packets []*Packet
	}
	restorer struct {
		rs    reedsolomon.Encoder
		total uint64
		data  uint64
	}
	dispatcher struct {
		size     uint64
		first    uint64
		last     uint64
		awaiting uint64
		chunks   []*Chunk
	}

	assemblyQueue chan *Packet
	restoreQueue  chan []*Packet
	dispatchQueue chan *Chunk
	returnQueue   chan []byte
}

func NewDecoder(dataParts, totalParts, assemblerLines, dispatcherSize uint64) (decoder *Decoder, err error) {
	if dataParts == 0 {
		return nil, errors.New("dataParts cannot be zero")
	}
	if totalParts < 2 {
		return nil, errors.New("totalParts cannot be less than 2")
	}

	decoder = new(Decoder)
	decoder.dispatcher.size = dispatcherSize
	decoder.dispatcher.awaiting = 1
	decoder.dispatcher.chunks = make([]*Chunk, dispatcherSize)

	decoder.restorer.rs, err = reedsolomon.New(int(dataParts), int(totalParts)-int(dataParts))
	if err != nil {
		return nil, err
	}
	decoder.restorer.data = dataParts
	decoder.restorer.total = totalParts

	decoder.assembler.size = assemblerLines * totalParts
	decoder.assembler.lines = assemblerLines
	decoder.assembler.found = make([]uint64, assemblerLines)
	decoder.assembler.csn = make([]uint64, assemblerLines)
	decoder.assembler.packets = make([]*Packet, assemblerLines*totalParts)

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

			if chunk.csn == 0 {
				log.Println("dropping invalid chunk: csn == 0")
				continue dispatch
			}

			if chunk.csn <= decoder.dispatcher.last {
				log.Println("detached chunk: csn:", chunk.csn)
				continue dispatch
			}

			decoder.dispatcher.first = max(decoder.dispatcher.first, chunk.csn)

			if chunk.csn != decoder.dispatcher.awaiting {
				var at = chunk.csn % uint64(decoder.dispatcher.size)
				if decoder.dispatcher.chunks[at] != nil {
					log.Println("lost: csn:", decoder.dispatcher.awaiting)
					decoder.returnQueue <- decoder.dispatcher.chunks[at].Data()
					decoder.dispatcher.last = decoder.dispatcher.chunks[at].csn
					decoder.dispatcher.awaiting = decoder.dispatcher.chunks[at].csn + 1

					decoder.dispatcher.chunks[at] = chunk
				} else {
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
				if decoder.dispatcher.chunks[at].csn != decoder.dispatcher.awaiting {
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
				if uint64(packets[i].psn) > decoder.restorer.total {
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

			if packet.csn == 0 {
				log.Println("dropping invalid packet: csn == 0")
				continue assembly
			}

			if packet.csn <= decoder.assembler.last {
				log.Println("detached packet: csn:", packet.csn, "psn:", packet.psn)
				continue assembly
			}

			decoder.assembler.first = max(decoder.assembler.first, packet.csn)

			var atPacket = (packet.csn*decoder.restorer.total + uint64(packet.psn)) % decoder.assembler.size
			var atChunk = packet.csn % decoder.assembler.lines
			// happy path: pushing into existing chunk line
			if decoder.assembler.found[atChunk] != 0 && decoder.assembler.csn[atChunk] == packet.csn {
				if uint64(packet.psn) > decoder.restorer.total {
					log.Println("invalid psn:", packet.psn)
					continue assembly
				}
				decoder.assembler.packets[atPacket] = packet
				decoder.assembler.found[atChunk]++

				if decoder.assembler.found[atChunk] >= decoder.restorer.data {
					var chunkStart = packet.csn * decoder.restorer.total % decoder.assembler.size
					var chunkEnd = ((packet.csn + 1) * decoder.restorer.total) % decoder.assembler.size
					if chunkEnd == 0 {
						chunkEnd = decoder.assembler.size
					}
					var data = make([]*Packet, decoder.restorer.total)
					copy(data, decoder.assembler.packets[chunkStart:chunkEnd])
					decoder.restoreQueue <- data
					decoder.assembler.last = packet.csn
					for at := chunkStart; chunkStart < chunkEnd; chunkStart++ {
						decoder.assembler.packets[at] = nil
					}
					decoder.assembler.found[atChunk] = 0
					decoder.assembler.csn[atChunk] = 0
				}

				continue assembly
			}

			// happy path: creating new chunk at unoccupied index
			if decoder.assembler.found[atChunk] == 0 {
				decoder.assembler.packets[atPacket] = packet
				decoder.assembler.csn[atChunk] = packet.csn
				decoder.assembler.found[atChunk] = 1

				continue assembly
				// we ignore cases where total = 1 since it is noop in terms of fec
			}

			if decoder.assembler.found[atChunk] != 0 && decoder.assembler.csn[atChunk] != packet.csn {
				log.Println("dropping outdated chunk: csn:", decoder.assembler.csn[atChunk])
				var chunkStart = packet.csn * decoder.restorer.total % decoder.assembler.size
				var chunkEnd = (packet.csn*decoder.restorer.total + decoder.restorer.total) % decoder.assembler.size
				for at := chunkStart; at < chunkEnd; at++ {
					decoder.assembler.packets[at] = nil
				}
				decoder.assembler.packets[atPacket] = packet
				decoder.assembler.csn[atChunk] = packet.csn
				decoder.assembler.found[atChunk] = 1

				continue assembly
			}

			panic("why are we here?")
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
