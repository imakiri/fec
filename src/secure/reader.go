package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"github.com/go-faster/errors"
	"io"
	"log"
	"math/rand"
)

type Reader struct {
	size  uint16
	nonce []byte
	rand  *rand.Rand
	aead  cipher.AEAD
	in    io.Reader
	buf   []byte
}

func (r *Reader) Size() uint16 {
	return r.size + BlockSize + uint16(r.aead.NonceSize())
}

func NewReader(size uint16, secret []byte, in io.Reader) (*Reader, error) {
	if in == nil {
		return nil, errors.New("in is nil")
	}

	var c, err = aes.NewCipher(secret)
	if err != nil {
		return nil, errors.Wrap(err, "aes.NewCipher(secret)")
	}

	var reader = new(Reader)
	reader.aead, err = cipher.NewGCM(c)
	if err != nil {
		return nil, errors.Wrap(err, "cipher.NewGCM(c)")
	}

	reader.in = in
	reader.size = size
	reader.buf = make([]byte, reader.Size())
	reader.nonce = make([]byte, reader.aead.NonceSize())
	return reader, nil
}

func (r *Reader) Read(p []byte) (n int, err error) {
	var buf = make([]byte, r.Size())

	n, err = r.in.Read(buf)
	if err != nil {
		return n, err
	}
	if n != int(r.Size()) {
		return n, errors.Errorf("n != r.Size(), n = %d", n)
	}

	buf, err = r.aead.Open(nil, buf[:12], buf[12:], nil)
	if err != nil {
		log.Println(errors.Wrap(err, "aead.Open"))
		return 0, nil
	}
	n = copy(p, buf)
	if n != int(r.size) {
		return n, errors.Errorf("n != r.size, n: %d", n)
	}
	return n, nil
}

func (r *Reader) Close() error {
	if rr, ok := r.in.(io.Closer); ok {
		return rr.Close()
	}
	return nil
}
