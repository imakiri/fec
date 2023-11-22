package main

import (
	"context"
	"flag"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/gosuri/uilive"
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
	output *uilive.Writer

	readerOutput chan []byte
	writerOutput chan []byte

	secret []byte
}

func NewHandler(output *uilive.Writer, secret []byte) (*Handler, error) {
	if output == nil {
		return nil, errors.New("output cannot be nil")
	}

	var h = new(Handler)
	h.readerOutput = make(chan []byte, 2)
	h.writerOutput = make(chan []byte, 2)
	h.secret = secret
	h.output = output

	go func() {
		var line []byte
		var err error
		for {
			line = <-h.readerOutput
			_, err = output.Newline().Write(line)
			if err == io.EOF {
				return
			}

			line = <-h.writerOutput
			_, err = output.Newline().Write(line)
			if err == io.EOF {
				return
			}
		}
	}()

	return h, nil
}

func (h *Handler) Reader(server *net.UDPConn) (reader io.ReadCloser, err error) {
	reporter, err := fec.NewReporter(h.readerOutput, "in ")
	if err != nil {
		return nil, errors.Wrap(err, "fec.NewReporter")
	}

	reader, err = observer.NewReader(server, reporter, time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "observer.NewWriter")
	}

	//reader = unit.NewReader(reader, codec.PacketSize, true, false)

	if h.secret != nil {
		reader, err = secure.NewReader(codec.PacketSize, h.secret, reader)
		if err != nil {
			return nil, errors.Wrap(err, "secure.NewReader")
		}
	}

	return reader, nil
}

func (h *Handler) Writer(server *net.UDPConn) (writer io.WriteCloser, err error) {
	reporter, err := fec.NewReporter(h.writerOutput, "out")
	if err != nil {
		return nil, errors.Wrap(err, "fec.NewReporter")
	}

	writer, err = observer.NewWriter(server, reporter, time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "observer.NewWriter")
	}

	//writer = unit.NewWriter(writer, codec.PacketDataSize, false, true)

	if h.secret != nil {
		writer, err = secure.NewWriter(codec.PacketSize, h.secret, writer)
		if err != nil {
			return nil, errors.Wrap(err, "secure.NewWriter")
		}
	}

	return writer, nil
}

type Config struct {
	Mode   string
	Port   uint16
	PeerID uuid.UUID
}

func NewConfig(path string) (*Config, error) {
	f, err := toml.LoadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "toml.LoadFile(client.cfg)")
	}

	var cfg = new(struct {
		Mode   string `toml:"mode"`
		Port   uint16 `toml:"port"`
		PeerID string `toml:"peer_id"`
	})

	err = f.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "f.Unmarshal(cfg)")
	}

	var config = new(Config)
	config.Mode = cfg.Mode
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

	var output = uilive.New()
	output.RefreshInterval = time.Second
	output.Start()
	log.SetOutput(output.Bypass())

	var config, err = NewConfig(*cfg)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "NewConfig"))
	}

	//var secret, err = fec.Secret()
	//if err != nil {
	//	log.Fatalln(errors.Wrap(err, "secret()"))
	//}
	//log.Printf("your common secret: %x", secret)

	client, err := fec.NewClient(config.Mode, config.Port, 25565, "45.80.209.11")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	handler, err := NewHandler(output, nil)
	//handler, err := NewHandler(secret)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "NewHandler"))
	}

	err = client.Run(ctx, config.PeerID, handler)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "fec.NewClient"))
	}

	<-ctx.Done()
	output.Stop()
}
