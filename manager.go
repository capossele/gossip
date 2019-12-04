package gossip

import (
	"io"
	"net"
	"sync"

	pb "github.com/capossele/gossip/proto"

	"github.com/capossele/gossip/transport"
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/pkg/errors"
)

var log = logger.NewLogger("Gossip")

type Manager struct {
	neighborhood map[string]*Neighbor
	lock         sync.RWMutex
	trans        *transport.TransportTCP
}

type Neighbor struct {
	peer *peer.Peer
	conn *transport.Connection
}

func (m *Manager) addNeighbor(peer *peer.Peer, handshake func(*peer.Peer) (*transport.Connection, error)) {
	m.lock.RLock()
	if _, ok := m.neighborhood[peer.ID().String()]; ok {
		m.lock.RUnlock()
		return
	}
	m.lock.RUnlock()

	neighbor := &Neighbor{
		peer: peer,
	}

	conn, err := handshake(peer)
	if err != nil {
		// TODO: handle error, maybe drop peer
	}
	neighbor.conn = conn

	m.lock.Lock()
	m.neighborhood[peer.ID().String()] = neighbor
	m.lock.Unlock()
}

func (m *Manager) deleteNeighbor(neighbor *Neighbor) {
	m.lock.RLock()
	if _, ok := m.neighborhood[neighbor.peer.ID().String()]; !ok {
		m.lock.RUnlock()
		return
	}
	m.lock.RUnlock()

	m.lock.Lock()
	delete(m.neighborhood, neighbor.peer.ID().String())
	m.lock.Unlock()
}

func (m *Manager) readLoop(neighbor *Neighbor) {

	for {
		data, err := neighbor.conn.Read()
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			log.Debug("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			// TODO: maybe close the connection and remove this neighbor?
			if err != io.EOF {
				log.Warning("read error", "err", err)
			}
			log.Debug("reading stopped")
			return
		}
		if err := m.handlePacket(data, neighbor); err != nil {
			log.Warning("failed to handle packet", "from", neighbor.peer.ID().String(), "err", err)
		}
	}
}

func (m *Manager) handlePacket(data []byte, neighbor *Neighbor) error {

	switch pb.MType(data[0]) {

	// Incoming Transaction
	case pb.MTransaction:
		m := new(pb.Transaction)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		// do something

	// Incoming Transaction request
	case pb.MTransactionRequest:
		m := new(pb.TransactionRequest)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		// do something

	default:
		return nil
	}

	return nil

}
