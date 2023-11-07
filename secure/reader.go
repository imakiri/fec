package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"github.com/go-faster/errors"
	"io"
	"math/rand"
	"time"
)

type Reader struct {
	unitSize uint16
	rand     *rand.Rand
	aead     cipher.AEAD
	in       io.ReadCloser
	buf      []byte
}

func NewReader(unitSize uint16, secret []byte, in io.ReadCloser) (*Reader, error) {
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

	if (int(unitSize)+reader.aead.NonceSize())%BlockSize != 0 {
		return nil, errors.Errorf("invalid unit size: %d", unitSize)
	}

	reader.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	reader.in = in
	reader.buf = make([]byte, int(unitSize)+reader.aead.NonceSize())
	return reader, nil
}

func (r Reader) Read(p []byte) (n int, err error) {
	n, err = r.in.Read(r.buf)
	if err != nil {
		return n, err
	}
	if n != int(r.unitSize)+r.aead.NonceSize() {
		return n, errors.Errorf("n != len(cipherText+12), n: %d, r.unitSize: %d", n, r.unitSize)
	}

	var nonce, cipherText = r.buf[:r.aead.NonceSize()], r.buf[r.aead.NonceSize():]

	var plainText []byte
	plainText, err = r.aead.Open(cipherText[:0], nonce, cipherText, nil)
	n = copy(p, plainText)
	if n != int(r.unitSize) {
		return n, errors.Errorf("n != r.unitSize, n: %d", n)
	}
	return n, nil
}

func (r Reader) Close() error {
	return r.in.Close()
}
