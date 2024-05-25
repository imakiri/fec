package src

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gofrs/uuid/v5"
	codec2 "github.com/imakiri/stream/src/codec"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const udpMax = 1500

var UID = uuid.Must(uuid.FromString("1751b2a1-0ffd-44fe-8a2e-d3f153125c43")).Bytes()

type Stat struct {
	ID       uint64
	Sent     int64
	Received int64
}

type Client struct {
	stats *os.File

	portCaller   uint16
	portListener uint16

	serverPort uint16
	serverAddr string

	acceptedLocalAddrMu *sync.Cond
	acceptedLocalAddr   *net.UDPAddr

	decoder *codec2.Decoder
	encoder *codec2.Encoder

	writer io.WriteCloser
	reader io.ReadCloser

	serverConn   *net.UDPConn
	callerConn   *net.UDPConn
	listenerConn *net.UDPConn
}

func (client *Client) connect(ctx context.Context, peerID uuid.UUID) error {
	var ok bool

	{
		var localConfig = net.ListenConfig{
			Control:   nil,
			KeepAlive: 0,
		}

		var listener, err = localConfig.ListenPacket(ctx, "udp4", fmt.Sprintf("127.0.0.1:%d", client.portListener))
		if err != nil {
			return errors.Wrap(err, "connect: localConfig.ListenPacket")
		}

		client.listenerConn, ok = listener.(*net.UDPConn)
		if !ok {
			return errors.New("connect: listener.(*net.UDPConn) is not ok")
		}

		log.Println("local listener at", client.listenerConn.LocalAddr().String())
		client.listenerConn.SetWriteBuffer(20 * (codec2.PacketV2Size + 12))
	}
	{
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

		var caller, err = dialer.DialContext(ctx, "udp4", fmt.Sprintf("127.0.0.1:%d", client.portCaller))
		if err != nil {
			return errors.Wrap(err, "connect: dialer.DialContext")
		}

		var ok bool
		client.callerConn, ok = caller.(*net.UDPConn)
		if !ok {
			return errors.New("connect: caller.(*net.UDPConn) is not ok")
		}

		log.Println("local caller to", client.callerConn.RemoteAddr().String())
		client.callerConn.SetWriteBuffer(20 * (codec2.PacketV2Size + 12))
	}

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

	client.serverConn.SetReadBuffer(20 * (codec2.PacketV2Size + 12))
	go func() {
		<-ctx.Done()
		client.serverConn.Close()
		client.callerConn.Close()
		client.listenerConn.Close()
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
	var decoderIn, decoderOut, _ = client.decoder.Decode(ctx)
	//routeIn:
	go func() {
		var buf []byte
		for {
			select {
			case <-ctx.Done():
				return
			default:
				buf = make([]byte, udpMax)
				n, err := client.reader.Read(buf)
				if err != nil {
					log.Printf("routeIn: reader.Read: %v", err)
					return
				}
				select {
				case <-ctx.Done():
					return
				case decoderIn <- buf[:n]:
				}
			}
		}
	}()
	go func() {
		var buf []byte
		var err error
		for {
			select {
			case <-ctx.Done():
				return
			case buf = <-decoderOut:
				_, err = client.callerConn.Write(buf)
				if err != nil {
					log.Printf("routeIn: callerConn.Write: %v", err)
					return
				}

				if client.acceptedLocalAddr != nil {
					_, err = client.listenerConn.WriteToUDP(buf, client.acceptedLocalAddr)
					if err != nil {
						log.Printf("routeIn: callerConn.Write: %v", err)
						return
					}
				}
			}
		}
	}()
}

func (client *Client) routeOut(ctx context.Context) {
	var encodeIn, encoderOut, _ = client.encoder.Encode(ctx)
	//routeOut:
	go func() {
		var buf []byte
		var err error
		var addr *net.UDPAddr
		var n int
		for {
			select {
			case <-ctx.Done():
				return
			default:
				buf = make([]byte, udpMax)
				n, addr, err = client.listenerConn.ReadFromUDP(buf)
				if err != nil {
					log.Printf("routeOut: listenerConn.ReadFromUDP: %v", err)
					return
				}

				client.acceptedLocalAddr = addr

				select {
				case <-ctx.Done():
					return
				case encodeIn <- buf[:n]:
				}
			}
		}
	}()
	go func() {
		var buf = make([]byte, udpMax)
		var err error
		for {
			select {
			case <-ctx.Done():
				return
			case buf = <-encoderOut:
				_, err = client.writer.Write(buf)
				if err != nil {
					log.Printf("routeOut: writer.Write: %v", err)
					return
				}
			}
		}
	}()
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

	var i int
	for err = errors.New(""); err != nil; i++ {
		client.stats, err = os.Create(fmt.Sprintf("stats_%d", i))
	}

	go client.routeIn(ctx)
	go client.routeOut(ctx)
	return nil
}

type Config struct {
	Peer struct {
		ID           string
		Mode         string
		PortCaller   uint16
		PortListener uint16
	}
	Server struct {
		Port uint16
		Addr string
	}
	Fec struct {
		DataParts  uint64
		TotalParts uint64
	}
	Encoder struct {
		DispatcherTimeout uint64
		DispatcherSize    uint64
	}
	Decoder struct {
		AssemblerSize  uint64
		DispatcherSize uint64
	}
}

func NewClient(cfg *Config) (*Client, error) {
	var client = new(Client)
	var err error
	client.encoder, err = codec2.NewEncoder(
		time.Duration(cfg.Encoder.DispatcherTimeout)*time.Millisecond,
		cfg.Encoder.DispatcherSize,
		cfg.Fec.DataParts,
		cfg.Fec.TotalParts,
	)
	if err != nil {
		return nil, errors.Wrap(err, "codec.NewEncoder")
	}
	client.decoder, err = codec2.NewDecoder(
		cfg.Fec.DataParts,
		cfg.Fec.TotalParts,
		cfg.Decoder.AssemblerSize,
		cfg.Decoder.DispatcherSize,
	)
	if err != nil {
		return nil, errors.Wrap(err, "codec.NewEncoder")
	}
	client.serverPort = cfg.Server.Port
	client.serverAddr = cfg.Server.Addr
	client.portCaller = cfg.Peer.PortCaller
	client.portListener = cfg.Peer.PortListener
	client.acceptedLocalAddrMu = sync.NewCond(new(sync.Mutex))
	return client, nil
}
