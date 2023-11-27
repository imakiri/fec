package main

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/gosuri/uilive"
	"github.com/imakiri/stream/src"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"
)

type Peer struct {
	id   uuid.UUID
	addr *net.UDPAddr
}

type Server struct {
	port uint16

	peers  [2]Peer
	server *net.UDPConn

	logger              *uilive.Writer
	totalReceivedBefore uint64
	totalReceived       *uint64
	totalSentBefore     uint64
	totalSent           *uint64
}

func NewServer(port uint16, logger *uilive.Writer) *Server {
	var server = new(Server)
	server.logger = logger
	server.port = port
	return server
}

func (s *Server) waitForSenderAddr(ctx context.Context, senderConn *net.UDPConn) *net.UDPAddr {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			hello := make([]byte, 20)
			_, senderAddr, err := senderConn.ReadFromUDP(hello)
			if err != nil {
				log.Println(errors.Wrap(err, "conn.ReadFromUDP"))
				continue
			}
			if string(hello) == "hello from sender---" {
				log.Println("sender addr", senderAddr.String())
				return senderAddr
			}
			//log.Println("unknown hello", string(hello))
		}
	}
}

func (s *Server) waitForReceiverAddr(ctx context.Context, receiverConn *net.UDPConn) *net.UDPAddr {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			hello := make([]byte, 20)
			_, receiverAddr, err := receiverConn.ReadFromUDP(hello)
			if err != nil {
				log.Println(errors.Wrap(err, "conn.ReadFromUDP"))
				continue
			}
			if string(hello) == "hello from receiver-" {
				log.Println("receiver addr", receiverAddr.String())
				return receiverAddr
			}
			//log.Println("unknown hello", string(hello))
		}
	}
}

func (s *Server) setRoute(peerID uuid.UUID, addr *net.UDPAddr) {
	switch {
	case s.peers[0].id == peerID:
		log.Printf("set new addr %s for peer %s", addr.AddrPort(), peerID)
		s.peers[0].addr = addr
	case s.peers[1].id == peerID:
		log.Printf("set new addr %s for peer %s", addr.AddrPort(), peerID)
		s.peers[1].addr = addr
	case s.peers[0].id == uuid.Nil:
		log.Printf("set new peet: id %s addr %s", peerID, addr.AddrPort())
		s.peers[0].addr = addr
		s.peers[0].id = peerID
	case s.peers[1].id == uuid.Nil:
		log.Printf("set new peet: id %s addr %s", peerID, addr.AddrPort())
		s.peers[1].addr = addr
		s.peers[1].id = peerID
	default:
		log.Println("no more room for new peers")
	}
}

func (s *Server) checkHandshake(buf []byte) uuid.UUID {
	if len(buf) < 32 {
		return uuid.Nil
	}

	for i := 0; i < 16; i++ {
		if buf[i] != src.UID[i] {
			return uuid.Nil
		}
	}

	return uuid.FromBytesOrNil(buf[16:32])
}

func (s *Server) route(addr *net.UDPAddr) *net.UDPAddr {
	if s.peers[0].addr.AddrPort().String() == addr.AddrPort().String() {
		return s.peers[1].addr
	}
	if s.peers[1].addr.AddrPort().String() == addr.AddrPort().String() {
		return s.peers[0].addr
	}
	return nil
}

func (s *Server) serve(ctx context.Context) {
	var buf = make([]byte, udpMax)
serve:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, addr, err := s.server.ReadFromUDP(buf)
			if err != nil {
				log.Println(errors.Wrap(err, "serve: s.server.ReadFrom"))
				continue serve
			}
			atomic.AddUint64(s.totalReceived, uint64(n))

			var peerID = s.checkHandshake(buf)
			if peerID != uuid.Nil {
				log.Printf("incomming handshake from %s", addr.AddrPort().String())
				s.setRoute(peerID, addr)

				m, err := s.server.WriteToUDP(buf, addr)
				if err != nil {
					log.Println(errors.Wrap(err, "serve: handshake s.server.WriteToUDP"))
					continue serve
				}
				if m < 32 {
					log.Println(errors.Wrap(err, "serve: handshake ack failed"))
					continue serve
				}

				log.Println("handshake done")
				continue serve
			}

			var toAddr = s.route(addr)
			if toAddr == nil {
				log.Println(errors.Wrap(err, "serve: missing second peer"))
				continue serve
			}

			m, err := s.server.WriteToUDP(buf[:n], toAddr)
			if err != nil {
				log.Println(errors.Wrap(err, "serve: s.server.WriteToUDP"))
				continue serve
			}
			if n != m {
				log.Printf("routeOut: read %d written %d", n, m)
				continue serve
			}
			atomic.AddUint64(s.totalSent, uint64(n))
			continue serve
		}
	}
}

const addr = "127.0.0.1:10900"

func (s *Server) Serve(ctx context.Context) error {
	s.totalReceived = new(uint64)
	s.totalSent = new(uint64)

	var config = net.ListenConfig{
		Control:   nil,
		KeepAlive: 0,
	}

	var server, err = config.ListenPacket(ctx, "udp4", addr)
	if err != nil {
		return errors.Wrap(err, "config.ListenPacket")
	}

	var ok bool
	s.server, ok = server.(*net.UDPConn)
	if !ok {
		return errors.New("server.(*net.UDPConn): not ok")
	}
	go func() {
		<-ctx.Done()
		s.server.Close()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var totalReceived = atomic.LoadUint64(s.totalReceived)
				var totalSent = atomic.LoadUint64(s.totalSent)

				if int64(totalReceived)-int64(s.totalReceivedBefore) > 0 {
					_, _ = fmt.Fprintf(s.logger.Newline(), "incoming speed: %6d KBit/sec, outgoing speed: %6d KBit/sec, transmited: %6d MB\n",
						8*(totalReceived-s.totalReceivedBefore)/(1<<10), 8*(totalSent-s.totalSentBefore)/(1<<10), totalReceived/(1<<20))
				} else if totalReceived-s.totalReceivedBefore == 0 {
				} else {
					log.Printf("transmited: %d MBit\n", 8*(s.totalReceivedBefore-totalReceived)/(1<<20))
				}

				s.totalReceivedBefore = totalReceived
				s.totalSentBefore = totalSent
				time.Sleep(time.Second)
			}
		}
	}()

	go s.serve(ctx)
	return nil
}

const udpMax = 1472

func main() {
	var logger = uilive.New()
	logger.Start()
	logger.RefreshInterval = time.Second
	log.SetOutput(logger.Bypass())

	var server = NewServer(25565, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var err = server.Serve(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	<-ctx.Done()
	logger.Stop()
}
