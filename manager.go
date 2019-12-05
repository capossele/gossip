package gossip

import (
	"net"
	"sync"

	pb "github.com/capossele/gossip/proto"
	"github.com/capossele/gossip/transport"
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	maxConnectionAttempts = 3
)

type GetTransaction func(txHash []byte) ([]byte, error)

type Manager struct {
	trans          *transport.TransportTCP
	log            *zap.SugaredLogger
	getTransaction GetTransaction

	wg sync.WaitGroup

	mu        sync.RWMutex
	neighbors map[peer.ID]*neighbor
	running   bool
}

type neighbor struct {
	peer *peer.Peer
	conn *transport.Connection
}

func NewManager(t *transport.TransportTCP, log *zap.SugaredLogger, f GetTransaction) *Manager {
	m := &Manager{
		trans:          t,
		log:            log,
		getTransaction: f,
		neighbors:      make(map[peer.ID]*neighbor),
	}

	m.running = true
	return m
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.running = false
	// close all connections
	for _, n := range m.neighbors {
		n.conn.Close()
	}
	m.mu.Unlock()

	m.wg.Wait()
}

func (m *Manager) getNeighbors(ids ...peer.ID) []*neighbor {
	if len(ids) > 0 {
		return m.getNeighborsById(ids)
	}
	return m.getAllNeighbors()
}

func (m *Manager) getAllNeighbors() []*neighbor {
	m.mu.Lock()
	result := make([]*neighbor, 0, len(m.neighbors))
	for _, n := range m.neighbors {
		result = append(result, n)
	}
	m.mu.Unlock()

	return result
}

func (m *Manager) getNeighborsById(ids []peer.ID) []*neighbor {
	result := make([]*neighbor, 0, len(ids))

	m.mu.RLock()
	for _, id := range ids {
		if n, ok := m.neighbors[id]; ok {
			result = append(result, n)
		}
	}
	m.mu.RUnlock()

	return result
}

func (m *Manager) RequestTransaction(data []byte, to ...peer.ID) {
	req := &pb.TransactionRequest{}
	err := proto.Unmarshal(data, req)
	if err != nil {
		m.log.Warnw("Data to send is not a Transaction Request", "err", err)
	}
	msg := marshal(req)

	m.send(msg, to...)
}

func (m *Manager) Send(data []byte, to ...peer.ID) {
	tx := &pb.Transaction{}
	err := proto.Unmarshal(data, tx)
	if err != nil {
		m.log.Warnw("Data to send is not a Transaction", "err", err)
	}
	msg := marshal(tx)

	m.send(msg, to...)
}

func (m *Manager) send(msg []byte, to ...peer.ID) {
	neighbors := m.getNeighbors(to...)

	for _, n := range neighbors {
		m.log.Debugw("Sending", "to", n.peer.ID(), "msg", msg)
		err := n.conn.Write(msg)
		if err != nil {
			m.log.Debugw("send error", "err", err)
		}
	}
}

func (m *Manager) addNeighbor(peer *peer.Peer, handshake func(*peer.Peer) (*transport.Connection, error)) error {
	var (
		err  error
		conn *transport.Connection
	)
	for i := 0; i < maxConnectionAttempts; i++ {
		conn, err = handshake(peer)
		if err == nil {
			break
		}
	}

	// could not add neighbor
	if err != nil {
		m.log.Debugw("addNeighbor failed", "peer", peer.ID(), "err", err)
		Events.DropNeighbor.Trigger(&DropNeighborEvent{Peer: peer})
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		disconnect(conn)
		return ErrClosed
	}
	if _, ok := m.neighbors[peer.ID()]; ok {
		disconnect(conn)
		return ErrDuplicateNeighbor
	}

	// add the neighbor
	n := &neighbor{
		peer: peer,
		conn: conn,
	}
	m.neighbors[peer.ID()] = n
	go m.readLoop(n)

	return nil
}

func (m *Manager) deleteNeighbor(peer *peer.Peer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.neighbors[peer.ID()]; !ok {
		return
	}

	n := m.neighbors[peer.ID()]
	delete(m.neighbors, peer.ID())
	disconnect(n.conn)
}

func (m *Manager) readLoop(n *neighbor) {
	m.wg.Add(1)
	defer m.wg.Done()

	for {
		data, err := n.conn.Read()
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			m.log.Debugw("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			m.log.Debugw("read error", "err", err)
			n.conn.Close() // just make sure that the connection is closed as fast as possible
			m.deleteNeighbor(n.peer)
			return
		}

		if err := m.handlePacket(data, n); err != nil {
			m.log.Warnw("failed to handle packet", "id", n.peer.ID(), "err", err)
		}
	}
}

func (m *Manager) handlePacket(data []byte, n *neighbor) error {
	switch pb.MType(data[0]) {

	// Incoming Transaction
	case pb.MTransaction:
		msg := new(pb.Transaction)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		m.log.Debugw("Received Transaction", "data", msg.GetBody())
		Events.NewTransaction.Trigger(&NewTransactionEvent{Body: msg.GetBody(), Peer: n.peer})

	// Incoming Transaction request
	case pb.MTransactionRequest:
		msg := new(pb.TransactionRequest)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		m.log.Debugw("Received Tx Req", "data", msg.GetHash())
		// do something
		tx, err := m.getTransaction(msg.GetHash())
		if err != nil {
			m.log.Debugw("Tx not available", "tx", msg.GetHash())
		} else {
			m.log.Debugw("Tx found", "tx", tx)
			m.Send(tx, n.peer.ID())
		}

	default:
	}

	return nil
}

func marshal(msg pb.Message) []byte {
	mType := msg.Type()
	if mType > 0xFF {
		panic("invalid message")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		panic("invalid message")
	}
	return append([]byte{byte(mType)}, data...)
}

func disconnect(conn *transport.Connection) {
	conn.Close()
	Events.DropNeighbor.Trigger(&DropNeighborEvent{Peer: conn.Peer()})
}
