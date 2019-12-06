package transport

import (
	"net"

	"github.com/iotaledger/autopeering-sim/peer"
)

type Connection struct {
	net.Conn
	peer *peer.Peer
}

func newConnection(c net.Conn, p *peer.Peer) *Connection {
	return &Connection{
		Conn: c,
		peer: p,
	}
}

func (c *Connection) Peer() *peer.Peer {
	return c.peer
}
