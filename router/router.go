package router

import (
	"github.com/go-faster/errors"
	"net"
)

type Service struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

func NewService(src *net.UDPConn, addr *net.UDPAddr) (*Service, error) {
	if src == nil {
		return nil, errors.New("conn is nil")
	}
	if addr == nil {
		return nil, errors.New("addr is nil")
	}
	var service = new(Service)
	service.addr = addr
	service.conn = src
	return service, nil
}

func (s *Service) Read(p []byte) (n int, err error) {
	var addr *net.UDPAddr
	var buf = make([]byte, len(p))

	for {
		n, addr, err = s.conn.ReadFromUDP(buf)
		if err != nil {
			return 0, err
		}
		if addr.String() == s.addr.String() {
			break
		}
	}

	return copy(p, buf), nil
}

func (s *Service) Close() error {
	return s.conn.Close()
}
