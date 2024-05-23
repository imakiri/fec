package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/imakiri/stream/src"
	codec2 "github.com/imakiri/stream/src/codec"
	"github.com/imakiri/stream/src/secure"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

func connect(ctx context.Context, peerID uuid.UUID, addr string, port uint16) (conn *net.UDPConn, err error) {
	var dialer = net.Dialer{
		Timeout:        0,
		Deadline:       time.Time{},
		LocalAddr:      nil,
		FallbackDelay:  0,
		KeepAlive:      0,
		Resolver:       nil,
		Control:        nil,
		ControlContext: nil,
	}

	connection, err := dialer.DialContext(ctx, "udp4", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, errors.Wrap(err, "connect: dialer.DialContext")
	}

	conn, ok := connection.(*net.UDPConn)
	if !ok {
		return nil, errors.New("connect: connection.(*net.UDPConn) is not ok")
	}

	conn.SetReadBuffer(20 * (codec2.PacketV2Size + 12))
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	var handshake = append(src.UID, peerID.Bytes()...)
	n, err := connection.Write(handshake)
	if err != nil {
		return nil, errors.Wrap(err, "connect: connection.Write")
	}
	if n != 32 {
		return nil, errors.Errorf("connect: connection.Write: written %d bytes", n)
	}

	var buf = make([]byte, 1472)
	n, err = connection.Read(buf)
	if err != nil {
		return nil, errors.Wrap(err, "connect: connection.Read")
	}
	if n < 32 {
		return nil, errors.Errorf("connect: connection.Read: read %d bytes", n)
	}

	if !bytes.Equal(buf[:32], handshake) {
		return nil, errors.Errorf("connect: failed: %s", buf)
	}

	log.Println("server at", conn.RemoteAddr().String())
	log.Printf("handshake done")
	return conn, nil
}

func main() {
	var mode = flag.String("mode", "writer", "stat client mode")
	var speed = flag.Int64("speed", 5000, "K byte per sec")
	var rawPeerID = flag.String("id", "", "peer id")
	var addr = flag.String("addr", "", "server address")
	var port = flag.Uint64("port", 25565, "server address")
	flag.Parse()

	peerID, err := uuid.FromString(*rawPeerID)
	if err != nil {
		log.Fatalln(err)
	}

	var ctx = context.Background()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	conn, err := connect(ctx, peerID, *addr, uint16(*port))
	if err != nil {
		log.Fatalln(err)
	}

	secret, err := hex.DecodeString("c0cfb869f0515d63ff9b3eb6158f15b60ce86189a670154f1ac98fd826fa31f9")
	if err != nil {
		log.Fatalln(err)
	}

	switch *mode {
	case "writer-enc":
		go func() {
			var writer, err = secure.NewWriter(996, secret, conn)
			if err != nil {
				log.Fatalln(err)
			}
			var buf = make([]byte, 996)
			var i uint64
			var sleep = time.Second / time.Duration(*speed)
			for ; ; time.Sleep(sleep) {
				select {
				case <-ctx.Done():
					return
				default:
					binary.LittleEndian.AppendUint64(buf[0:0:996], i)
					binary.LittleEndian.AppendUint64(buf[8:8:996], uint64(time.Now().UnixNano()))
					n, err := rand.Read(buf[16:996])
					if err != nil {
						log.Fatalln(err)
					}
					if n != 996-16 {
						log.Fatalln("writer: n != 996-16", n)
					}

					n, err = writer.Write(buf)
					if err != nil {
						log.Fatalln(err)
					}
					if n < 996 {
						log.Fatalln("writer: n < 996", n)
					}
					i++
				}
			}
		}()
	case "writer":
		go func() {
			var buf = make([]byte, 1024)
			var i uint64
			var sleep = time.Second / time.Duration(*speed)
			for ; ; time.Sleep(sleep) {
				select {
				case <-ctx.Done():
					return
				default:
					binary.LittleEndian.AppendUint64(buf[0:0:1024], i)
					binary.LittleEndian.AppendUint64(buf[8:8:1024], uint64(time.Now().UnixNano()))
					n, err := rand.Read(buf[16:1024])
					if err != nil {
						log.Fatalln(err)
					}
					if n != 1024-16 {
						log.Fatalln("writer: n != 1024-16", n)
					}

					n, err = conn.Write(buf)
					if err != nil {
						log.Fatalln(err)
					}
					if n != 1024 {
						log.Fatalln("writer: n != 1024", n)
					}
					i++
				}
			}
		}()
	case "reader":
		received, err := os.OpenFile("received", os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			log.Fatalln(err)
		}

		go func() {
			var i uint64
			var timestamp uint64
			var buf = make([]byte, 1472)
			for {
				select {
				case <-ctx.Done():
					received.Sync()
					received.Close()
					return
				default:
					n, err := conn.Read(buf)
					if err != nil {
						log.Fatalln(err)
					}
					if n < 8 {
						log.Fatalln("reader: n < 8", n)
					}
					i = binary.LittleEndian.Uint64(buf)
					timestamp = binary.LittleEndian.Uint64(buf[8:])
					n, err = fmt.Fprintf(received, "%d %d %d\n", i, timestamp, time.Now().UnixNano())
					if err != nil {
						log.Fatalln(err)
					}
					if n == 0 {
						log.Fatalln("reader: n == 0")
					}
				}
			}
		}()
	default:
		log.Fatalln("invalid mode", *mode)
	}

	<-ctx.Done()
}
