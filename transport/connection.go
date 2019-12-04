package transport

import (
	"net"

	"github.com/iotaledger/autopeering-sim/peer"
)

type connection struct {
	peer *peer.Peer
	conn net.Conn
}

func newConnection(p *peer.Peer, c net.Conn) *connection {
	return &connection{
		peer: p,
		conn: c,
	}
}

func (c *connection) Close() {
	c.conn.Close()
}

func (c *connection) Read() ([]byte, error) {
	b := make([]byte, MaxPacketSize)
	n, err := c.conn.Read(b)
	if err != nil {
		return nil, err
	}

	return b[:n], nil
}

func (c *connection) Write(b []byte) error {
	_, err := c.conn.Write(b)
	return err
}
