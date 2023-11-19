package main

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/imakiri/fec"
	"log"
	"os"
	"os/signal"
)

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

	err = client.Run(ctx, secret)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	<-ctx.Done()
}
