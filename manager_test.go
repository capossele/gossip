package gossip

import (
	"log"
	"sync"
	"testing"

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

func newTest(t require.TestingT, name string, address string) (*Manager, func(), *peer.Peer) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal("peering", address, db)
	require.NoError(t, err)
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", address))

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
	_, teardown, _ := newTest(t, "A", "127.0.0.1:0")
	teardown()
}

func TestConnect(t *testing.T) {
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
	}()
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
	}()

	wg.Wait()
	assert.Equal(t, mgrA.neighborhood.Len(), 1)
	assert.Equal(t, mgrA.neighborhood.Len(), 1)
}

func TestSend(t *testing.T) {
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
	}()
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
	}()

	wg.Wait()

	tx := &pb.Transaction{
		Body: []byte("Hello!"),
	}
	b, err := proto.Marshal(tx)
	assert.NoError(t, err)

	wg.Add(1)
	Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		assert.Equal(t, tx.GetBody(), ev.Body)
		wg.Done()

	}))

	mgrA.Send(b)

	wg.Wait()
}
