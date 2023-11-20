package main

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/imakiri/fec"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"
)

type Server struct {
	port uint16

	peers  [2]*net.UDPAddr
	server *net.UDPConn

	totalReceivedBefore uint64
	totalReceived       *uint64
	totalSentBefore     uint64
	totalSent           *uint64
}

func NewServer(port uint16) *Server {
	var server = new(Server)
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

func (s *Server) setRoute(addr *net.UDPAddr) {
	switch {
	case s.peers[0] == nil || s.peers[0].AddrPort().Addr().String() == addr.AddrPort().Addr().String():
		s.peers[0] = addr
	case s.peers[1] == nil || s.peers[1].AddrPort().Addr().String() == addr.AddrPort().Addr().String():
		s.peers[1] = addr
	default:
		s.peers[0] = addr
	}
}

func (s *Server) checkHandshake(buf []byte) bool {
	for i := 0; i < 16; i++ {
		if buf[i] != fec.UID[i] {
			return false
		}
	}
	return true
}

func (s *Server) route(addr *net.UDPAddr) *net.UDPAddr {
	if s.peers[0].AddrPort().String() == addr.AddrPort().String() {
		return s.peers[1]
	}
	if s.peers[1].AddrPort().String() == addr.AddrPort().String() {
		return s.peers[0]
	}
	return nil
}

func (s *Server) serve(ctx context.Context) {
	var buf = make([]byte, upd_packet_size)
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

			if s.checkHandshake(buf) {
				s.setRoute(addr)
			}

			var toAddr = s.route(addr)
			if toAddr == nil {
				log.Println(errors.Wrap(err, "serve: unknown destination"))
				continue serve
			}

			m, err := s.server.WriteToUDP(buf, toAddr)
			if err != nil {
				log.Println(errors.Wrap(err, "serve: s.server.WriteToUDP"))
				continue serve
			}
			if n != m {
				log.Println(errors.Wrap(err, "serve: n != m"))
				continue serve
			}
			atomic.AddUint64(s.totalSent, uint64(n))
			continue serve
		}
	}
}

func (s *Server) Serve(ctx context.Context) error {
	s.totalReceived = new(uint64)
	s.totalSent = new(uint64)

	var config = net.ListenConfig{
		Control:   nil,
		KeepAlive: 0,
	}

	var server, err = config.ListenPacket(ctx, "udp4", "45.80.209.11:25565")
	if err != nil {
		return errors.Wrap(err, "config.ListenPacket")
	}

	var ok bool
	s.server, ok = server.(*net.UDPConn)
	if !ok {
		return errors.New("server.(*net.UDPConn): not ok")
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var totalReceived = atomic.LoadUint64(s.totalReceived)
				var totalSent = atomic.LoadUint64(s.totalSent)

				if int64(totalReceived)-int64(s.totalReceivedBefore) > 0 {
					log.Printf("incoming speed: %6d KBit/sec, outgoing speed: %6d KBit/sec\n",
						8*(totalReceived-s.totalReceivedBefore)/(1<<10), 8*(totalSent-s.totalSentBefore)/(1<<10))
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

const upd_packet_size = 1472
const buffer_size = 10000 * upd_packet_size

func main() {
	var server = NewServer(25565)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var err = server.Serve(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	<-ctx.Done()
}
