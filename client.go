package fec

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/imakiri/fec/codec"
	"github.com/imakiri/fec/secure"
	"log"
	"net"
	"time"
)

var uid = uuid.Must(uuid.FromString("1751b2a1-0ffd-44fe-8a2e-d3f153125c43"))

type Client struct {
	localPort  uint16
	serverPort uint16
	serverAddr string

	acceptedLocalAddr *net.UDPAddr
	secureWriter      *secure.Writer
	secureReader      *secure.Reader
	serverConn        *net.UDPConn
	localConn         *net.UDPConn
}

func (client *Client) connect(ctx context.Context) error {
	var localConfig = net.ListenConfig{
		Control:   nil,
		KeepAlive: 0,
	}

	var listener, err = localConfig.ListenPacket(ctx, "udp4", fmt.Sprintf("127.0.0.1:%d", client.localPort))
	if err != nil {
		return errors.Wrap(err, "connect: localConfig.ListenPacket")
	}

	var ok bool
	client.localConn, ok = listener.(*net.UDPConn)
	if !ok {
		return errors.New("connect: listener.(*net.UDPConn) is not ok")
	}

	client.localConn.SetWriteBuffer(20 * (codec.PacketSize + 12))
	log.Println("local at", client.localConn.LocalAddr().String())

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
	log.Println("server at", client.serverConn.RemoteAddr().String())

	n, err := connection.Write(uid.Bytes())
	if err != nil {
		return errors.Wrap(err, "connect: connection.Write")
	}
	if n != 16 {
		return errors.Errorf("connect: connection.Write: written %d bytes", n)
	}

	var buf = make([]byte, 16)
	n, err = connection.Read(buf)
	if err != nil {
		return errors.Wrap(err, "connect: connection.Read")
	}
	if n != 16 {
		return errors.Errorf("connect: connection.Read: read %d bytes", n)
	}

	if !bytes.Equal(buf, uid.Bytes()) {
		return errors.Errorf("connect: failed: %s", buf)
	}

	return nil
}

func (client *Client) routeIn(ctx context.Context) {
	var buf = make([]byte, codec.PacketDataSize)
	//routeIn:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := client.secureReader.Read(buf)
			if err != nil {
				log.Printf("routeIn: secureReader.Read: %v", err)
				return
			}

			for client.acceptedLocalAddr == nil {
			}

			m, err := client.localConn.WriteToUDP(buf, client.acceptedLocalAddr)
			if err != nil {
				log.Printf("routeIn: localConn.WriteToUDP: %v", err)
				return
			}
			if n != m {
				log.Println("routeIn: read and written are different")
				return
			}
		}
	}

}

func (client *Client) routeOut(ctx context.Context) {
	var buf = make([]byte, codec.PacketDataSize)
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
			if client.acceptedLocalAddr == nil {
				client.acceptedLocalAddr = addr
			} else {
				if client.acceptedLocalAddr.AddrPort().String() != addr.AddrPort().String() {
					log.Printf("routeOut: wrong local address: expecting %v, got %v", client.acceptedLocalAddr.AddrPort().String(), addr.AddrPort().String())
					continue routeOut
				}
			}

			m, err := client.secureWriter.Write(buf)
			if err != nil {
				log.Printf("routeOut: serverConn.Write: %v", err)
				return
			}
			if n != m {
				log.Println("routeOut: read and written are different")
				return
			}
		}
	}

}

func (client *Client) Run(ctx context.Context, secret []byte) error {
	var err error

	err = client.connect(ctx)
	if err != nil {
		return errors.Wrap(err, "client.connect")
	}

	client.secureWriter, err = secure.NewWriter(codec.PacketSize, secret, client.serverConn)
	if err != nil {
		return errors.Wrap(err, "secureWriter.NewWriter")
	}

	client.secureReader, err = secure.NewReader(codec.PacketSize, secret, client.serverConn)
	if err != nil {
		return errors.Wrap(err, "secureWriter.NewWriter")
	}

	go client.routeIn(ctx)
	go client.routeOut(ctx)
	return nil
}

func NewClient(localPort uint16, serverPort uint16, serverAddr string) (*Client, error) {
	var router = new(Client)
	router.serverPort = serverPort
	router.serverAddr = serverAddr
	router.localPort = localPort
	return router, nil
}
