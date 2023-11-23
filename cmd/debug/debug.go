package main

import (
	"context"
	"flag"
	"github.com/go-faster/errors"
	"github.com/gosuri/uilive"
	"github.com/pelletier/go-toml"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

const (
	Listener = "listener"
	Caller   = "caller"
)

type Config struct {
	Mode string
	Port uint16
}

func NewConfig(path string) (*Config, error) {
	f, err := toml.LoadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "toml.LoadFile(client.cfg)")
	}

	var cfg = new(struct {
		Mode string `toml:"mode"`
		Port uint16 `toml:"port"`
	})

	err = f.Unmarshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "f.Unmarshal(cfg)")
	}

	var config = new(Config)
	config.Mode = cfg.Mode
	config.Port = cfg.Port
	return config, nil
}

const timeFormat = "2006-01-02T15:04:05.00000"

func serveCaller(ctx context.Context, port uint16) error {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: int(port),
	})
	if err != nil {
		return err
	}

	for ; ; time.Sleep(20 * time.Millisecond) {
		select {
		case <-ctx.Done():
			return nil
		default:
			var now, _ = time.Now().MarshalBinary()
			_, err = conn.Write(now)
			if err != nil {
				return err
			}
		}
	}
}

func serveListener(ctx context.Context, port uint16) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: int(port),
	})
	if err != nil {
		return err
	}

	for ; ; time.Sleep(time.Millisecond) {
		select {
		case <-ctx.Done():
			return nil
		default:
			var buf = make([]byte, 1472)
			_, err = conn.Read(buf)
			if err != nil {
				return err
			}
			var now = time.Now()
			var t = time.Time{}
			err = t.UnmarshalBinary(buf[:15])
			if err != nil {
				return err
			}
			log.Printf("sent: %s, received: %s, delta: %d ms\n", t.Format(timeFormat), now.Format(timeFormat), now.Sub(t).Milliseconds())
		}
	}
}

func main() {
	var cfg = flag.String("cfg", "debug.cfg", "path to toml config file")
	flag.Parse()

	var config, err = NewConfig(*cfg)
	if err != nil {
		log.Fatalln(err)
	}

	var output = uilive.New()
	output.RefreshInterval = time.Second
	output.Start()
	log.SetOutput(output.Bypass())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	switch config.Mode {
	case Caller:
		err = serveCaller(ctx, config.Port)
	case Listener:
		err = serveListener(ctx, config.Port)
	}
	if err != nil {
		log.Fatalln(err)
	}

	<-ctx.Done()
}
