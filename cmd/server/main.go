package main

import (
	"context"
	"github.com/go-faster/errors"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	incomingPort, outgoingPort uint16
	totalReceivedBefore        uint64
	totalReceived              *uint64
	totalSentBefore            uint64
	totalSent                  *uint64
}

func NewServer(incomingPort, outgoingPort uint16) *Server {
	var server = new(Server)
	server.incomingPort = incomingPort
	server.outgoingPort = outgoingPort
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

func (s *Server) server(ctx context.Context) error {
	s.totalReceived = new(uint64)
	s.totalSent = new(uint64)

	senderConn, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   nil,
		Port: int(s.incomingPort),
		Zone: "",
	})
	if err != nil {
		return errors.Wrap(err, "sender listener")
	}
	senderConn.SetReadBuffer(buffer_size)
	log.Println("awaiting sender at", senderConn.LocalAddr().String())
	defer senderConn.Close()

	receiverConn, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   nil,
		Port: int(s.outgoingPort),
		Zone: "",
	})
	if err != nil {
		return errors.Wrap(err, "receiver listener")
	}
	receiverConn.SetWriteBuffer(buffer_size)
	log.Println("awaiting receiver at", receiverConn.LocalAddr().String())
	defer receiverConn.Close()

	go func() {
		<-ctx.Done()
		receiverConn.Close()
		senderConn.Close()
	}()

	var senderAddr *net.UDPAddr
	var wg = new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		senderAddr = s.waitForSenderAddr(ctx, senderConn)
		wg.Done()
	}()

	var receiverAddr *net.UDPAddr
	go func() {
		receiverAddr = s.waitForReceiverAddr(ctx, receiverConn)
		wg.Done()
	}()
	wg.Wait()

	var buf = make([]byte, upd_packet_size)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			senderConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, addr, err := senderConn.ReadFromUDP(buf)
			if err != nil {
				log.Println(errors.Wrap(err, "sender.ReadFromUDP(buf)"))
				return nil
			}
			if !addr.IP.Equal(senderAddr.IP) {
				log.Println(errors.Errorf("!addr.IP.Equal(senderAddr.IP), %s", addr.IP.String()))
				continue
			}
			atomic.AddUint64(s.totalReceived, uint64(n))

			receiverConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			n, err = receiverConn.WriteToUDP(buf, receiverAddr)
			if err != nil {
				log.Println(errors.Wrap(err, "receiver.WriteToUDP(buf, receiverAddr)"))
				return nil
			}
			atomic.AddUint64(s.totalSent, uint64(n))
		}
	}
}

func (s *Server) Serve(ctx context.Context) error {
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

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var err = s.server(ctx)
			if err != nil {
				log.Println(err)
			}
		}
	}

}

const upd_packet_size = 1472
const buffer_size = 10000 * upd_packet_size

func main() {
	var server = NewServer(25565, 25566)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var err = server.Serve(ctx)
	if err != nil {
		log.Fatalln(err)
	}
}
