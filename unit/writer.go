package unit

import (
	"bytes"
	"github.com/go-faster/errors"
	"io"
)

type Writer struct {
	dst      io.WriteCloser
	buf      *bytes.Buffer
	n        int
	size     int
	fixed    bool
	complete bool
}

func NewWriter(dst io.WriteCloser, size int, fixed, complete bool) *Writer {
	var r = new(Writer)
	r.buf = bytes.NewBuffer(make([]byte, 0, size))
	r.dst = dst
	r.size = size
	r.fixed = fixed
	r.complete = complete
	return r
}

func (r *Writer) Write(b []byte) (int, error) {
	defer func() {
		r.n = 0
	}()

	if r.fixed && len(b) != r.size {
		return 0, errors.New("len(b) != r.size")
	}

	var n, err = r.dst.Write(b)
	if n == r.size || !r.complete {
		return n, err
	}

	r.n = n
	for r.n < r.size {
		var n, err = r.dst.Write(b[n:])
		if err != nil {
			return r.n, errors.Wrap(err, "r.dst.Write(b[n:])")
		}
		r.n += n
	}

	return r.size, nil
}

func (r *Writer) Close() error {
	r.buf.Reset()
	return r.dst.Close()
}
