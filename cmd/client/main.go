package main

import (
	"context"
	"flag"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/imakiri/fec"
	"github.com/imakiri/fec/codec"
	"github.com/imakiri/fec/observer"
	"github.com/imakiri/fec/secure"
	"github.com/pelletier/go-toml"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

type Handler struct {
	secret []byte
}

func NewHandler(secret []byte) (*Handler, error) {
	var h = new(Handler)
	h.secret = secret
	return h, nil
}

func (h *Handler) Reader(server *net.UDPConn) (reader io.ReadCloser, err error) {
	reader, err = observer.NewReader(server, fec.Reporter, time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "observer.NewWriter")
	}

	if h.secret != nil {
		reader, err = secure.NewReader(codec.PacketSize, h.secret, reader)
		if err != nil {
			return nil, errors.Wrap(err, "secure.NewReader")
		}
	}

	return reader, nil
}

func (h *Handler) Writer(server *net.UDPConn) (writer io.WriteCloser, err error) {
	writer, err = observer.NewWriter(server, fec.Reporter, time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "observer.NewWriter")
	}

	if h.secret != nil {
		writer, err = secure.NewWriter(codec.PacketSize, h.secret, writer)
		if err != nil {
			return nil, errors.Wrap(err, "secure.NewWriter")
		}
	}

	return writer, nil
}

type Config struct {
	Port   uint16
	PeerID uuid.UUID
}

func NewConfig(path string) (*Config, error) {
	f, err := toml.LoadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "toml.LoadFile(client.cfg)")
	}

	var cfg = new(struct {
		Port   uint16 `toml:"port"`
		PeerID string `toml:"peer_id"`
	})

	err = f.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "f.Unmarshal(cfg)")
	}

	var config = new(Config)
	config.Port = cfg.Port
	config.PeerID, err = uuid.FromString(cfg.PeerID)
	if err != nil {
		return nil, errors.Wrap(err, "uuid.FromString(cfg.PeerID)")
	}

	return config, nil
}

func main() {
	var cfg = flag.String("cfg", "client.cfg", "path to toml config file")
	flag.Parse()

	var config, err = NewConfig(*cfg)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "NewConfig"))
	}

	//var secret, err = fec.Secret()
	//if err != nil {
	//	log.Fatalln(errors.Wrap(err, "secret()"))
	//}
	//log.Printf("your common secret: %x", secret)

	client, err := fec.NewClient(config.Port, 25565, "45.80.209.11")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	handler, err := NewHandler(nil)
	//handler, err := NewHandler(secret)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "NewHandler"))
	}

	err = client.Run(ctx, config.PeerID, handler)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	<-ctx.Done()
}
