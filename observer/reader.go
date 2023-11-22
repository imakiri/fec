package observer

import (
	"github.com/go-faster/errors"
	"io"
	"math/bits"
	"sync/atomic"
	"time"
)

type Reader struct {
	done        chan struct{}
	src         io.ReadCloser
	rep         Reporter
	totalBefore uint64
	total       *uint64
	interval    time.Duration
}

func NewReader(src io.ReadCloser, rep Reporter, interval time.Duration) (*Reader, error) {
	if src == nil {
		return nil, errors.New("dst is nil")
	}
	if rep == nil {
		return nil, errors.New("reporter is nil")
	}

	var reader = new(Reader)
	reader.done = make(chan struct{})
	reader.src = src
	reader.rep = rep
	reader.total = new(uint64)
	reader.interval = interval

	go func() {
		for {
			select {
			case <-reader.done:
				return
			default:
				var total = atomic.LoadUint64(reader.total)
				var delta, underflow = bits.Sub64(total, reader.totalBefore, 0)
				if underflow == 0 {
					rep.Report(total, delta)
				} else {
					rep.Report(total, 0)
				}
				reader.totalBefore = total
				time.Sleep(reader.interval)
			}
		}
	}()

	return reader, nil
}

func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.src.Read(p)
	//log.Printf("read %x\n", p)
	atomic.AddUint64(r.total, uint64(n))
	return n, err
}

func (r *Reader) Close() error {
	close(r.done)
	return r.src.Close()
}
