package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"github.com/go-faster/errors"
	"io"
)

type Writer struct {
	size  uint16
	nonce []byte
	rand  io.Reader
	aead  cipher.AEAD
	out   io.Writer
	buf   []byte
}

func (w *Writer) Size() uint16 {
	return w.size + uint16(len(w.nonce)) + BlockSize
}

func NewWriter(size uint16, secret []byte, out io.Writer) (*Writer, error) {
	if out == nil {
		return nil, errors.New("out is nil")
	}

	var c, err = aes.NewCipher(secret)
	if err != nil {
		return nil, errors.Wrap(err, "aes.NewCipher(secret)")
	}

	var writer = new(Writer)
	writer.aead, err = cipher.NewGCM(c)
	if err != nil {
		return nil, errors.Wrap(err, "cipher.NewGCM(c)")
	}

	writer.size = size
	writer.nonce = make([]byte, writer.aead.NonceSize())
	writer.rand = rand.Reader

	writer.out = out
	writer.buf = make([]byte, writer.Size())
	return writer, nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if len(p) != int(w.size) {
		return 0, errors.New("len(p) != int(w.size)")
	}

	n, err = w.rand.Read(w.nonce)
	if err != nil {
		return 0, errors.Wrap(err, "rand.Read(w.nonce)")
	}
	if n != len(w.nonce) {
		return 0, errors.New("n != len(w.nonce)")
	}

	var data = make([]byte, w.Size())
	n = copy(data, w.nonce)
	if n != 12 {
		return 0, errors.New("n != 12")
	}

	w.aead.Seal(data[12:12], w.nonce, p, nil)

	n, err = w.out.Write(data)
	if err != nil {
		return n, err
	}
	if n != len(data) {
		return n, errors.Errorf("n != len(data), n: %d, cipher text: %d", n, len(data))
	}

	return n, nil
}

func (w *Writer) Close() error {
	if ww, ok := w.out.(io.Closer); ok {
		return ww.Close()
	}
	return nil
}
