package traversal

import (
	"github.com/gofrs/uuid/v5"
	"github.com/imakiri/fec"
	"github.com/imakiri/fec/buffer"
	"github.com/imakiri/fec/erres"
	"log"
)

type Invocation struct {
	Req []byte
	Res func([]byte)
}

type Server struct {
	peers [2]uuid.UUID
	box   [2]*buffer.Ring[[]byte]
}

func NewServer(bufSize uint64) (*Server, error) {
	var server = new(Server)
	server.box[0] = buffer.NewRing(0, make([][]byte, bufSize))
	server.box[1] = buffer.NewRing(0, make([][]byte, bufSize))
	return server, nil
}

const ErrInvalidData erres.Error = "invalid data"
const ErrMaxPeers erres.Error = "maximum peers reached"
const ErrPeerNotFound erres.Error = "peer not found"
const ErrInvalidMode erres.Error = "invalid mode"

func (s *Server) serveHandshake(inv Invocation) error {
	var data = inv.Req[1:]
	if len(data) < 32 {
		return ErrInvalidData
	}

	for i := 0; i < 16; i++ {
		if data[i] != fec.UID[i] {
			return ErrInvalidData
		}
	}

	var peerID = uuid.FromBytesOrNil(data[16:32])
	if peerID == uuid.Nil {
		return ErrInvalidData
	}

	switch {
	case s.peers[0] == uuid.Nil:
		s.peers[0] = peerID
		log.Printf("peer.0: %s", peerID.String())
	case s.peers[1] == uuid.Nil:
		s.peers[1] = peerID
		log.Printf("peer.1: %s", peerID.String())
	default:
		return ErrMaxPeers
	}

	inv.Res(inv.Req)
	return nil
}

func (s *Server) serveSend(inv Invocation) error {
	var data = inv.Req[1:]
	if len(data) < 32 {
		return ErrInvalidData
	}

	var peerID = uuid.FromBytesOrNil(data[0:16])
	if peerID == uuid.Nil {
		return ErrInvalidData
	}

	switch peerID {
	case s.peers[0]:
		s.box[0].Write(false, data[16:])
	case s.peers[1]:
		s.box[1].Write(false, data[16:])
	default:
		return ErrPeerNotFound
	}

	return nil
}

func (s *Server) Serve(inv Invocation) error {
	if len(inv.Req) < 1 {
		return ErrInvalidData
	}

	switch mode(inv.Req[0]) {
	case Handshake:
		return s.serveHandshake(inv)
	case Send:
		return s.serveSend(inv)
	default:
		return ErrInvalidMode
	}
}
