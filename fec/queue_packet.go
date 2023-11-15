package fec

type PacketQueue struct {
	size       uint32
	minPackets uint32
	maxPackets uint32

	csn struct {
		lastPopped uint64
		lastPushed uint64
	}

	lastIndex int
	// inlined []element
	order  []uint32
	csns   []uint64
	chunks [][]*Packet

	done         bool
	processQueue chan []*Packet
}

func NewPacketQueue(minPackets, maxPackets, size uint32) (queue *PacketQueue, shutdown func(), err error) {
	queue = new(PacketQueue)
	queue.size = size
	queue.minPackets = minPackets
	queue.maxPackets = maxPackets
	queue.order = make([]uint32, size)
	queue.csns = make([]uint64, size)
	queue.chunks = make([][]*Packet, size)
	queue.processQueue = make(chan []*Packet, size)
	queue.done = false
	for i := range queue.chunks {
		queue.chunks[i] = make([]*Packet, maxPackets)
	}
	shutdown = func() {
		queue.done = true
		close(queue.processQueue)
	}

	return queue, shutdown, nil
}

func (q *PacketQueue) Digest(packet *Packet) {
	if packet.csn <= q.csn.lastPopped || q.done || packet == nil {
		return
	}

	if packet.csn > q.csn.lastPushed {
		// pushing new chunk
		for i := uint32(0); i < q.size; i++ {
			if q.order[i] == 0 {
				q.csn.lastPopped = q.csns[i]
				// dropping the oldest chunk and replacing it with a new one, containing single packet
				q.order[i] = q.size - 1
				q.csns[i] = packet.csn
				q.chunks[i] = q.chunks[i][0:1]
				// pushing new packet
				q.chunks[i][0] = packet
			} else {
				// due to the push we shift the order of all other packets by -1
				q.order[i]--
			}
		}
		q.csn.lastPushed = packet.csn
		return
	}

	var index uint32
	for i := uint32(0); i < q.size; i++ {
		// looking for an index related to the incoming packet at which its chunk is stored
		if q.csns[i] == packet.csn {
			q.chunks[i] = append(q.chunks[i], packet)
			if uint32(len(q.chunks[i])) == q.minPackets {
				index = i
				break // breaking the loop when we found a complete chunk
			}
			return
		}
	}

	q.processQueue <- q.chunks[index]

	for i := uint32(0); i < q.size; i++ {
		if i == index || q.order[i] > q.order[index] || q.csns[i] == 0 {
			continue
		}
		q.order[i]++
	}

	q.csns[index] = 0
	q.order[index] = 0
	q.chunks[index] = q.chunks[index][0:0]
	q.csn.lastPopped = packet.csn
}
