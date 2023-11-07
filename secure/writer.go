package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"github.com/go-faster/errors"
	"io"
	"math/rand"
	"time"
)

type Writer struct {
	unitSize uint16
	rand     *rand.Rand
	aead     cipher.AEAD
	out      io.WriteCloser
	buf      []byte
}

func NewWriter(unitSize uint16, secret []byte, out io.WriteCloser) (*Writer, error) {
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

	if (int(unitSize)+writer.aead.NonceSize())%BlockSize != 0 {
		return nil, errors.Errorf("invalid unit size: %d", unitSize)
	}

	writer.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	writer.out = out
	writer.buf = make([]byte, int(unitSize)+writer.aead.NonceSize())
	return writer, nil
}

func (w Writer) Write(p []byte) (n int, err error) {
	var nonce = make([]byte, w.aead.NonceSize())
	n, err = io.ReadFull(w.rand, nonce)
	if err != nil {
		return 0, err
	}
	if n != len(nonce) {
		return 0, errors.New("nonce is not complete")
	}

	var cipherText = w.aead.Seal(p[:0], nonce, p, nil)

	n, err = w.out.Write(cipherText)
	if err != nil {
		return n, err
	}
	if n != len(cipherText) {
		return n, errors.Errorf("n != len(cipherText), n: %d, cipher text: %d", n, len(cipherText))
	}

	return n, nil
}

func (w Writer) Close() error {
	return w.out.Close()
}
