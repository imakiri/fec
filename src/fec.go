package src

import (
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/go-faster/errors"
	"log"
	"math/rand"
	"time"
)

var curve = ecdh.P256()

func Secret() ([]byte, error) {
	var prk, err = curve.GenerateKey(rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return nil, errors.Wrap(err, "curve.GenerateKey")
	}

	fmt.Printf("your public key: %x\n", prk.PublicKey().Bytes())
	clipboard.WriteAll(hex.EncodeToString(prk.PublicKey().Bytes()))

	log.Print("enter others public key: ")
	var input string
	fmt.Scanln(&input)

	pukR, err := hex.DecodeString(input)
	if err != nil {
		return nil, errors.Wrap(err, "hex.DecodeString(input)")
	}

	puk, err := curve.NewPublicKey(pukR)
	if err != nil {
		return nil, errors.Wrap(err, "curve.NewPublicKey(pukR)")
	}

	secret, err := prk.ECDH(puk)
	if err != nil {
		return nil, errors.Wrap(err, "prk.ECDH(puk)")
	}

	return secret, nil
}
