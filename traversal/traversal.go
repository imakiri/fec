package traversal

import "github.com/gofrs/uuid/v5"

type mode byte

const (
	Handshake mode = 157
	Send      mode = 1
	Ask       mode = 131
	Response  mode = 2
)

type Packet interface {
	Mode() mode
}

type PacketHandshake struct {
	mode   mode
	key    uuid.UUID
	peerID uuid.UUID
}

func (p *PacketHandshake) Mode() mode {
	return p.mode
}

type PacketSend struct {
	mode    mode
	peerID  uuid.UUID
	payload []byte
}

func (p *PacketSend) Mode() mode {
	return p.mode
}

type PacketAsk struct {
	mode   mode
	peerID uuid.UUID
}

func (p *PacketAsk) Mode() mode {
	return p.mode
}

type PacketResponse struct {
	mode    mode
	peerID  uuid.UUID
	payload []byte
}

func (p *PacketResponse) Mode() mode {
	return p.mode
}
