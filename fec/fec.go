package fec

type Queue struct {
	data    [][]Packet
	csns    []uint64
	lastCSN uint64
}

func NewQueue(size int) (*Queue, error) {
	var queue = new(Queue)
	queue.data = make([][]Packet, size)
	return queue, nil
}

func (q *Queue) search(csn uint64) uint64 {
	for i := 0; i < len(q.data)
}
