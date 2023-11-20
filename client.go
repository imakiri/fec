package fec

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/imakiri/fec/codec"
	"io"
	"log"
	"net"
	"time"
)

const udpMax = 1472

var UID = uuid.Must(uuid.FromString("1751b2a1-0ffd-44fe-8a2e-d3f153125c43")).Bytes()

type Client struct {
	mode      mode
	localPort uint16

	serverPort uint16
	serverAddr string

	acceptedLocalAddr *net.UDPAddr

	writer io.WriteCloser
	reader io.ReadCloser

	serverConn *net.UDPConn
	localConn  *net.UDPConn
}

func (client *Client) connect(ctx context.Context, peerID uuid.UUID) error {
	var ok bool
	switch client.mode {
	case Listener:
		var localConfig = net.ListenConfig{
			Control:   nil,
			KeepAlive: 0,
		}

		var listener, err = localConfig.ListenPacket(ctx, "udp4", fmt.Sprintf("127.0.0.1:%d", client.localPort))
		if err != nil {
			return errors.Wrap(err, "connect: localConfig.ListenPacket")
		}

		client.localConn, ok = listener.(*net.UDPConn)
		if !ok {
			return errors.New("connect: listener.(*net.UDPConn) is not ok")
		}

		log.Println("local listener at", client.localConn.LocalAddr().String())
	case Caller:
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

		var dial, err = dialer.DialContext(ctx, "udp4", fmt.Sprintf("127.0.0.1:%d", client.localPort))
		if err != nil {
			return errors.Wrap(err, "connect: localConfig.ListenPacket")
		}

		var ok bool
		client.localConn, ok = dial.(*net.UDPConn)
		if !ok {
			return errors.New("connect: listener.(*net.UDPConn) is not ok")
		}

		log.Println("local caller to", client.localConn.RemoteAddr().String())
	}

	client.localConn.SetWriteBuffer(20 * (codec.PacketSize + 12))

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

	connection, err := dialer.DialContext(ctx, "udp4", fmt.Sprintf("%s:%d", client.serverAddr, client.serverPort))
	if err != nil {
		return errors.Wrap(err, "connect: dialer.DialContext")
	}

	client.serverConn, ok = connection.(*net.UDPConn)
	if !ok {
		return errors.New("connect: connection.(*net.UDPConn) is not ok")
	}

	client.serverConn.SetReadBuffer(20 * (codec.PacketSize + 12))
	go func() {
		<-ctx.Done()
		client.serverConn.Close()
		client.localConn.Close()
	}()

	var handshake = append(UID, peerID.Bytes()...)
	n, err := connection.Write(handshake)
	if err != nil {
		return errors.Wrap(err, "connect: connection.Write")
	}
	if n != 32 {
		return errors.Errorf("connect: connection.Write: written %d bytes", n)
	}

	var buf = make([]byte, udpMax)
	n, err = connection.Read(buf)
	if err != nil {
		return errors.Wrap(err, "connect: connection.Read")
	}
	if n < 32 {
		return errors.Errorf("connect: connection.Read: read %d bytes", n)
	}

	if !bytes.Equal(buf[:32], handshake) {
		return errors.Errorf("connect: failed: %s", buf)
	}

	log.Println("server at", client.serverConn.RemoteAddr().String())
	log.Printf("handshake done")
	return nil
}

func (client *Client) routeIn(ctx context.Context) {
	var buf = make([]byte, udpMax)
	//routeIn:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := client.reader.Read(buf)
			if err != nil {
				log.Printf("routeIn: secureReader.Read: %v", err)
				return
			}

			var addr *net.UDPAddr
			switch client.mode {
			case Caller:
				for client.acceptedLocalAddr == nil {
				}
				addr = client.acceptedLocalAddr
			case Listener:
				addr = &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: int(client.localPort),
					Zone: "",
				}
			}

			m, err := client.localConn.WriteToUDP(buf[:n], addr)
			if err != nil {
				log.Printf("routeIn: localConn.WriteToUDP: %v", err)
				return
			}
			if n != m {
				log.Printf("routeIn: read %d written %d", n, m)
				return
			}

		}
	}

}

func (client *Client) routeOut(ctx context.Context) {
	var buf = make([]byte, udpMax)
routeOut:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, addr, err := client.localConn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("routeOut: localConn.ReadFromUDP: %v", err)
				return
			}

			switch client.mode {
			case Caller:
				if client.acceptedLocalAddr == nil {
					client.acceptedLocalAddr = addr
				} else {
					if client.acceptedLocalAddr.AddrPort().String() != addr.AddrPort().String() {
						log.Printf("routeOut: wrong local address: expecting %v, got %v", client.acceptedLocalAddr.AddrPort().String(), addr.AddrPort().String())
						continue routeOut
					}
				}
			case Listener:
			}

			m, err := client.writer.Write(buf[:n])
			if err != nil {
				log.Printf("routeOut: serverConn.Write: %v", err)
				return
			}
			if n != m {
				log.Printf("routeOut: read %d written %d", n, m)
				return
			}
		}
	}
}

type Handler interface {
	Reader(server *net.UDPConn) (io.ReadCloser, error)
	Writer(server *net.UDPConn) (io.WriteCloser, error)
}

func (client *Client) Run(ctx context.Context, peerID uuid.UUID, handler Handler) error {
	var err error

	err = client.connect(ctx, peerID)
	if err != nil {
		return errors.Wrap(err, "client.connect")
	}

	client.writer, err = handler.Writer(client.serverConn)
	if err != nil {
		return errors.Wrap(err, "writer")
	}

	client.reader, err = handler.Reader(client.serverConn)
	if err != nil {
		return errors.Wrap(err, "reader")
	}

	go client.routeIn(ctx)
	go client.routeOut(ctx)
	return nil
}

type mode string

const (
	Caller   mode = "caller"
	Listener mode = "listener"
)

func NewClient(mode string, localPort uint16, serverPort uint16, serverAddr string) (*Client, error) {
	var router = new(Client)
	switch mode {
	case string(Caller):
		router.mode = Caller
	case string(Listener):
		router.mode = Listener
	default:
		return nil, errors.Errorf("invalid mode: %s", mode)
	}

	router.serverPort = serverPort
	router.serverAddr = serverAddr
	router.localPort = localPort
	return router, nil
}
