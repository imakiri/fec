package main

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/go-faster/errors"
	"haha/secure"
	"haha/unit"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"
)

const upd_packet_size = 1472
const buffer_size = 20 * upd_packet_size

func connect() (server, local *net.UDPConn, err error) {
	server, err = net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.ParseIP("45.80.209.11"),
		Port: 25565,
		Zone: "",
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "net.DialUDP")
	}
	server.SetWriteBuffer(buffer_size)
	log.Println("server at", server.RemoteAddr().String())

	localAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:25567")
	if err != nil {
		return nil, nil, errors.Wrap(err, "ResolveUDPAddr")
	}
	local, err = net.ListenUDP("udp4", localAddr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "DialUDP")
	}
	local.SetReadBuffer(buffer_size)
	log.Println("local at", local.LocalAddr().String())

	return server, local, nil
}

var curve = ecdh.P256()

func secret() ([]byte, error) {
	var prk, err = curve.GenerateKey(rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return nil, errors.Wrap(err, "curve.GenerateKey")
	}

	fmt.Printf("your public key: %x\n", prk.PublicKey().Bytes())
	clipboard.WriteAll(hex.EncodeToString(prk.PublicKey().Bytes()))

	log.Print("enter others public key: ")
	var input string
	fmt.Scanln(&input)

	puk_r, err := hex.DecodeString(input)
	if err != nil {
		return nil, errors.Wrap(err, "hex.DecodeString(input)")
	}

	puk, err := curve.NewPublicKey(puk_r)
	if err != nil {
		return nil, errors.Wrap(err, "curve.NewPublicKey(puk_r)")
	}

	secret, err := prk.ECDH(puk)
	if err != nil {
		return nil, errors.Wrap(err, "prk.ECDH(puk)")
	}

	return secret, nil
}

func main() {
	var scr, err = secret()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "secret()"))
	}

	log.Printf("your common secret: %x", scr)

	server, local, err := connect()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "connect()"))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		local.Close()
		server.Close()
	}()
	defer cancel()

	_, err = server.Write([]byte("hello from sender---"))
	if err != nil {
		log.Fatalln(errors.Wrap(err, "server.ReadFromUDP(buf)"))
	}
	log.Println("hello sent")

	log.Println("awaiting local data")
	local.Read(make([]byte, 16))
	log.Println("local data")

	var totalLocalBefore, totalServerBefore uint64
	var totalLocal, totalServer uint64
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var totalLocal = atomic.LoadUint64(&totalLocal)
				var totalServer = atomic.LoadUint64(&totalServer)
				log.Printf("local speed: %8d KBit/sec, server speed: %8d KBit/sec\n",
					8*(totalLocal-totalLocalBefore)/(1<<10), 8*(totalServer-totalServerBefore)/(1<<10))
				totalLocalBefore = totalLocal
				totalServerBefore = totalServer
				time.Sleep(time.Second)
			}
		}
	}()

	var c, _ = codec.NewCodec()
	var enc = c.Encoder(&totalLocal, &totalServer)

	var unitReader = unit.NewReader(local, enc.ChunkSize(), true)
	var unitWriter = unit.NewWriter(server, codec.PacketSize, false)

	secureWriter, err := secure.NewWriter(codec.PacketSize, scr, unitWriter)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "secure.NewWriter(fec.PacketSize, scr, unitWriter)"))
	}

	for {
		select {
		case <-ctx.Done():
			server.Close()
			local.Close()
			return
		default:
			var err = enc.Encode(ctx, unitReader, secureWriter)
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}
