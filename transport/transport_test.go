package transport

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
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

func newTest(t require.TestingT, name string) (*TransportTCP, func()) {
	l := logger.Named(name)
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)

	// enable TCP gossipping
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", "127.0.0.1:0"))

	trans, err := Listen(local, l)
	require.NoError(t, err)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, trans.LocalAddr().Network(), trans.LocalAddr().String()))

	teardown := func() {
		trans.Close()
		db.Close()
	}
	return trans, teardown
}

func getPeer(t *TransportTCP) *peer.Peer {
	return &t.local.Peer
}

func TestClose(t *testing.T) {
	_, teardown := newTest(t, "A")
	teardown()
}

func TestUnansweredAccept(t *testing.T) {
	transA, closeA := newTest(t, "A")
	defer closeA()

	_, err := transA.AcceptPeer(getPeer(transA))
	assert.Error(t, err)
}

func TestCloseWhileAccepting(t *testing.T) {
	transA, closeA := newTest(t, "A")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := transA.AcceptPeer(getPeer(transA))
		assert.Error(t, err)
	}()
	time.Sleep(graceTime)

	closeA()
	wg.Wait()
}

func TestUnansweredDial(t *testing.T) {
	transA, closeA := newTest(t, "A")
	defer closeA()

	// create peer with invalid gossip address
	services := getPeer(transA).Services().CreateRecord()
	services.Update(service.GossipKey, "tcp", "127.0.0.1:0")
	unreachablePeer := peer.NewPeer(getPeer(transA).PublicKey(), services)

	_, err := transA.DialPeer(unreachablePeer)
	assert.Error(t, err)
}

func TestConnect(t *testing.T) {
	transA, closeA := newTest(t, "A")
	defer closeA()
	transB, closeB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c, err := transB.AcceptPeer(getPeer(transA))
		assert.NoError(t, err)
		if assert.NotNil(t, c) {
			c.Close()
		}
	}()
	go func() {
		defer wg.Done()
		c, err := transA.DialPeer(getPeer(transB))
		assert.NoError(t, err)
		if assert.NotNil(t, c) {
			c.Close()
		}
	}()

	wg.Wait()
}

func TestWrongConnect(t *testing.T) {
	transA, closeA := newTest(t, "A")
	defer closeA()
	transB, closeB := newTest(t, "B")
	defer closeB()
	transC, closeC := newTest(t, "C")
	defer closeC()

	var wg sync.WaitGroup
	wg.Add(2)

	// a expects connection from B, but C is connecting
	go func() {
		defer wg.Done()
		_, err := transA.AcceptPeer(getPeer(transB))
		assert.Error(t, err)
	}()
	go func() {
		defer wg.Done()
		_, err := transC.DialPeer(getPeer(transA))
		assert.Error(t, err)
	}()

	wg.Wait()
}
