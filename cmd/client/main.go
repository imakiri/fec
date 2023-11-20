package main

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/imakiri/fec"
	"github.com/imakiri/fec/codec"
	"github.com/imakiri/fec/observer"
	"github.com/imakiri/fec/secure"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

func reader(secret []byte) func(server *net.UDPConn) (io.ReadCloser, error) {
	return func(server *net.UDPConn) (reader io.ReadCloser, err error) {
		reader, err = observer.NewReader(server, fec.Reporter, time.Second)
		if err != nil {
			return nil, errors.Wrap(err, "observer.NewWriter")
		}

		if secret != nil {
			reader, err = secure.NewReader(codec.PacketSize, secret, reader)
			if err != nil {
				return nil, errors.Wrap(err, "secure.NewReader")
			}
		}

		return reader, nil
	}
}

func writer(secret []byte) func(server *net.UDPConn) (io.WriteCloser, error) {
	return func(server *net.UDPConn) (writer io.WriteCloser, err error) {
		writer, err = observer.NewWriter(server, fec.Reporter, time.Second)
		if err != nil {
			return nil, errors.Wrap(err, "observer.NewWriter")
		}

		if secret != nil {
			writer, err = secure.NewWriter(codec.PacketSize, secret, writer)
			if err != nil {
				return nil, errors.Wrap(err, "secure.NewWriter")
			}
		}

		return writer, nil
	}
}

func main() {
	var secret, err = fec.Secret()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "secret()"))
	}
	log.Printf("your common secret: %x", secret)

	client, err := fec.NewClient(25565, 25565, "45.80.209.11")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	err = client.Run(ctx, reader(nil), writer(nil))
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	<-ctx.Done()
}
