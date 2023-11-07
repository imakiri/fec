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

const upd_packet_size = 1472

func main() {
	src, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 25568,
		Zone: "",
	})
	if err != nil {
		log.Fatalln(errors.Wrap(err, "net.DialUDP"))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var buf = make([]byte, upd_packet_size)
loop:
	for {
		select {
		case <-ctx.Done():
			src.Close()
			break loop
		default:
			_, _, err := src.ReadFromUDP(buf)
			if err != nil {
				log.Println(errors.Wrap(err, "income.ReadFromUDP(buf)"))
				continue
			}
			fmt.Printf("%x\n", buf)
		}
	}
}
