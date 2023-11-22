package observer

import (
	"github.com/go-faster/errors"
	"io"
	"math/bits"
	"sync/atomic"
	"time"
)

type Writer struct {
	done        chan struct{}
	dst         io.WriteCloser
	rep         Reporter
	totalBefore uint64
	total       *uint64
	interval    time.Duration
}

func NewWriter(dst io.WriteCloser, rep Reporter, interval time.Duration) (*Writer, error) {
	if dst == nil {
		return nil, errors.New("dst is nil")
	}
	if rep == nil {
		return nil, errors.New("reporter is nil")
	}

	var writer = new(Writer)
	writer.done = make(chan struct{})
	writer.dst = dst
	writer.rep = rep
	writer.total = new(uint64)
	writer.interval = interval

	go func() {
		for {
			select {
			case <-writer.done:
				return
			default:
				var total = atomic.LoadUint64(writer.total)
				var delta, underflow = bits.Sub64(total, writer.totalBefore, 0)
				if underflow == 0 {
					rep.Report(total, delta)
				} else {
					rep.Report(total, 0)
				}
				writer.totalBefore = total
				time.Sleep(writer.interval)
			}
		}
	}()

	return writer, nil
}

func (r *Writer) Write(p []byte) (n int, err error) {
	n, err = r.dst.Write(p)
	//log.Printf("write %x\n", p)
	atomic.AddUint64(r.total, uint64(n))
	return n, err
}

func (r *Writer) Close() error {
	close(r.done)
	return r.dst.Close()
}
