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

const graceTime = 5 * time.Millisecond

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()
}

func newTest(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", "localhost:0"))

	trans, err := transport.Listen(local, log)
	require.NoError(t, err)

	mgr := NewManager(trans, log)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, trans.LocalAddr().Network(), trans.LocalAddr().String()))

	teardown := func() {
		trans.Close()
		db.Close()
	}
	return mgr, teardown, &local.Peer
}

func TestClose(t *testing.T) {
	_, teardown, _ := newTest(t, "A")
	teardown()
}

func TestUnicast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	tx := &pb.Transaction{Body: []byte("Hello!")}

	triggered := make(chan struct{}, 1)
	Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		require.Empty(t, triggered) // only once
		assert.Equal(t, tx.GetBody(), ev.Body)
		triggered <- struct{}{}
	}))

	b, err := proto.Marshal(tx)
	require.NoError(t, err)
	mgrA.Send(b)

	// eventually the event should be triggered
	assert.Eventually(t, func() bool { _, ok := <-triggered; return ok }, time.Second, 10*time.Millisecond)
}

func TestBroadcast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()
	mgrC, closeC, peerC := newTest(t, "C")
	defer closeC()

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerC, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.addNeighbor(peerA, mgrC.trans.DialPeer)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	tx := &pb.Transaction{Body: []byte("Hello!")}

	triggered := make(chan struct{}, 2)
	Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		require.Less(t, len(triggered), 2) // triggered at most twice
		assert.Equal(t, tx.GetBody(), ev.Body)
		triggered <- struct{}{}
	}))

	b, err := proto.Marshal(tx)
	assert.NoError(t, err)
	// send the message to A and B
	mgrA.Send(b)

	// eventually the event should be triggered twice
	assert.Eventually(t, func() bool { return len(triggered) == 2 }, time.Second, 10*time.Millisecond)
}

func TestDropUnsuccessfulAccept(t *testing.T) {
	mgrA, closeA, _ := newTest(t, "A")
	defer closeA()
	_, closeB, peerB := newTest(t, "B")
	defer closeB()

	triggered := make(chan struct{}, 1)
	Events.DropNeighbor.Attach(events.NewClosure(func(ev *DropNeighborEvent) {
		require.Empty(t, triggered) // only once
		assert.Equal(t, peerB, ev.Peer)
		triggered <- struct{}{}
	}))

	err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
	assert.Error(t, err)

	// eventually the event should be triggered
	assert.Eventually(t, func() bool { _, ok := <-triggered; return ok }, time.Second, 10*time.Millisecond)
}
