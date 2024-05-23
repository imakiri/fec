package main

import (
	"context"
	"github.com/go-faster/errors"
	"log"
	"net"
	"os"
	"os/signal"
)

func run(ctx context.Context) error {
	var localConfig = net.ListenConfig{
		Control:   nil,
		KeepAlive: 0,
	}

	var listener, err = localConfig.ListenPacket(ctx, "udp4", "45.80.209.11:1090")
	if err != nil {
		return errors.Wrap(err, "connect: localConfig.ListenPacket")
	}

	var listenerConn, ok = listener.(*net.UDPConn)
	if !ok {
		return errors.New("connect: listener.(*net.UDPConn) is not ok")
	}

	log.Println("local listener at", listenerConn.LocalAddr().String())

	var buf = make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, addr, err := listenerConn.ReadFromUDP(buf)
			if err != nil {
				return errors.New("listenerConn.ReadFromUDP failed")
			}

			_, err = listenerConn.WriteTo(buf, addr)
			if err != nil {
				return errors.New("listenerConn.WriteTo failed")
			}
		}
	}
}

func main() {
	var ctx = context.Background()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	log.Fatalln(run(ctx))
}
