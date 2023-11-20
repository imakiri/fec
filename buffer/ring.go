package buffer

import (
	"sync"
)

type Ring[T any] struct {
	mu       *sync.Mutex
	newWrite *sync.Cond
	newRead  *sync.Cond

	buf   []T
	first uint64
	last  uint64
	size  uint64
}

func NewRing[T any](first uint64, buf []T) *Ring[T] {
	var mu = new(sync.Mutex)
	return &Ring[T]{
		mu:       mu,
		newWrite: sync.NewCond(mu),
		newRead:  sync.NewCond(mu),
		buf:      buf,
		first:    first,
		last:     0,
		size:     uint64(len(buf)),
	}
}

func (r *Ring[T]) Read(wait bool) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
read:
	if r.first-r.last == 0 {
		if !wait {
			var t T
			return t, false
		} else {
			r.newWrite.Wait()
			goto read
		}
	}

	r.last++
	defer r.newRead.Signal()
	return r.buf[r.last%r.size], true
}

func (r *Ring[T]) Write(overwrite bool, t T) {
	r.mu.Lock()
	defer r.mu.Unlock()
write:
	if r.first-r.last == r.size {
		if overwrite {
			r.first++
			r.last++
			r.buf[r.first%r.size] = t
			return
		} else {
			r.newRead.Wait()
			goto write
		}
	}

	r.first++
	defer r.newWrite.Signal()
	r.buf[r.first%r.size] = t
}
