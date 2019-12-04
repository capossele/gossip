package gossip

import (
	"log"
	"sync"
	"testing"
	"time"

	pb "github.com/capossele/gossip/proto"
	"github.com/capossele/gossip/transport"
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()
}
func testGetTransaction(h []byte) ([]byte, error) {
	return []byte("testTx"), nil
}

func newTest(t require.TestingT, name string, address string) (*Manager, func(), *peer.Peer) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal("peering", address, db)
	require.NoError(t, err)
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", address))

	trans, err := transport.Listen(local, log)
	require.NoError(t, err)

	mgr := NewManager(trans, log, testGetTransaction)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, trans.LocalAddr().Network(), trans.LocalAddr().String()))

	teardown := func() {
		trans.Close()
		db.Close()
	}
	return mgr, teardown, &local.Peer
}

func TestClose(t *testing.T) {
	_, teardown, _ := newTest(t, "A", "127.0.0.1:0")
	teardown()
}

func TestUnicast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A", "127.0.0.1:0")
	defer closeA()

	mgrB, closeB, peerB := newTest(t, "B", "127.0.0.1:0")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrA.neighborhood.Len())
	}()
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrB.neighborhood.Len())
	}()

	wg.Wait()

	tx := &pb.Transaction{
		Body: []byte("Hello!"),
	}
	b, err := proto.Marshal(tx)
	assert.NoError(t, err)

	sendChan := make(chan struct{})
	sendSuccess := false

	Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		logger.Debugw("New TX Event triggered", "data", ev.Body)
		assert.Equal(t, tx.GetBody(), ev.Body)
		sendChan <- struct{}{}
	}))

	mgrA.Send(b)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-sendChan:
		sendSuccess = true
	case <-timer.C:
		sendSuccess = false
	}

	assert.True(t, sendSuccess)
}

func TestBroadcast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A", "127.0.0.1:0")
	defer closeA()

	mgrB, closeB, peerB := newTest(t, "B", "127.0.0.1:0")
	defer closeB()

	mgrC, closeC, peerC := newTest(t, "C", "127.0.0.1:0")
	defer closeC()

	tx := &pb.Transaction{
		Body: []byte("Hello!"),
	}

	Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		logger.Debugw("New TX Event triggered", "data", ev.Body)
		assert.Equal(t, tx.GetBody(), ev.Body)
	}))

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
		err = mgrA.addNeighbor(peerC, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrA.neighborhood.Len())
	}()
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrB.neighborhood.Len())
	}()
	go func() {
		defer wg.Done()
		err := mgrC.addNeighbor(peerA, mgrC.trans.DialPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrC.neighborhood.Len())
	}()

	wg.Wait()

	b, err := proto.Marshal(tx)
	assert.NoError(t, err)

	mgrA.Send(b)

	time.Sleep(1 * time.Second)
}

func TestDrop(t *testing.T) {
	mgrA, closeA, _ := newTest(t, "A", "127.0.0.1:0")
	defer closeA()

	_, closeB, peerB := newTest(t, "B", "127.0.0.1:0")
	defer closeB()

	doneChan := make(chan struct{}, 2)
	dropSuccess := false

	Events.DropNeighbor.Attach(events.NewClosure(func(ev *DropNeighborEvent) {
		logger.Debugw("Drop Event triggered", "peer", ev.Peer)
		assert.Equal(t, peerB, ev.Peer)
		doneChan <- struct{}{}
	}))

	err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
	assert.Error(t, err)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-doneChan:
		logger.Debugw("Channel consumed")
		dropSuccess = true
	case <-timer.C:
		logger.Debugw("Timer triggered")
		dropSuccess = false
	}

	assert.True(t, dropSuccess)
}
