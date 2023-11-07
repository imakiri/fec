package unit

import (
	"bytes"
	"github.com/go-faster/errors"
	"io"
)

type Reader struct {
	src      io.ReadCloser
	buf      *bytes.Buffer
	n        int
	size     int
	complete bool
	adaptive bool
}

func NewReader(src io.ReadCloser, size int, complete, adaptive bool) *Reader {
	var r = new(Reader)
	r.buf = bytes.NewBuffer(make([]byte, 0, size))
	r.src = src
	r.size = size
	r.complete = complete
	r.adaptive = adaptive
	return r
}

func (r *Reader) Read(b []byte) (int, error) {
	defer func() {
		r.n = 0
	}()

	var n, err = r.buf.Read(b[:r.size])
	if n == r.size || !r.complete {
		return n, nil
	}

	r.n = n
	var buf []byte
	if r.adaptive {
		buf = make([]byte, r.size-n)
	} else {
		buf = make([]byte, r.size)
	}
	for r.n < r.size {
		var n, err = r.src.Read(buf)
		if err != nil {
			return r.n, errors.Wrap(err, "r.buf.ReadFrom(r.dst)")
		}

		r.buf.Write(buf[:n])
		r.n += n
	}

	n, err = r.buf.Read(b[n:r.size])
	if err != nil {
		return n, errors.Wrap(err, "r.buf.Read(b[n:r.size])")
	}

	return r.size, nil
}

func (r *Reader) Close() error {
	r.buf.Reset()
	return r.src.Close()
}
