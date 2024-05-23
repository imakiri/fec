package main

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"log"
	"net"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var localConfig = net.ListenConfig{
		Control:   nil,
		KeepAlive: 0,
	}

	var listener, err = localConfig.ListenPacket(ctx, "udp4", fmt.Sprintf("127.0.0.1:%d", 25566))
	if err != nil {
		log.Fatalln(errors.Wrap(err, "connect: localConfig.ListenPacket"))
	}

	var conn, ok = listener.(*net.UDPConn)
	if !ok {
		log.Fatalln(errors.New("connect: listener.(*net.UDPConn) is not ok"))
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		var buf = make([]byte, 1500)
		select {
		case <-ctx.Done():
			return
		default:
			var n, err = conn.Read(buf)
			if err != nil {
				return
			}

			fmt.Printf("%d %x\n", n, buf)
		}
	}
}
