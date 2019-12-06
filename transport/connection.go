package transport

import (
	"net"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
)

type Connection struct {
	net.Conn
	peer *peer.Peer
}

func newConnection(c net.Conn, p *peer.Peer) *Connection {
	// make sure the connection has no timeouts
	_ = c.SetDeadline(time.Time{})

	return &Connection{
		Conn: c,
		peer: p,
	}
}

func (c *Connection) Peer() *peer.Peer {
	return c.peer
}
