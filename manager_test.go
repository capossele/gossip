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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const graceTime = 5 * time.Millisecond

var (
	logger    *zap.SugaredLogger
	eventMock mock.Mock

	testTransaction = &pb.Transaction{
		Body: []byte("testTx"),
	}
)

func newTransactionEvent(ev *NewTransactionEvent) { eventMock.Called(ev) }
func dropNeighborEvent(ev *DropNeighborEvent)     { eventMock.Called(ev) }

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()

	// mock the events
	Events.NewTransaction.Attach(events.NewClosure(newTransactionEvent))
	Events.DropNeighbor.Attach(events.NewClosure(dropNeighborEvent))
}

func getTestTransaction([]byte) ([]byte, error) {
	return proto.Marshal(testTransaction)
}

func newTest(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", "localhost:0"))

	trans, err := transport.Listen(local, log)
	require.NoError(t, err)

	mgr := NewManager(trans, log, getTestTransaction)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, trans.LocalAddr().Network(), trans.LocalAddr().String()))

	teardown := func() {
		trans.Close()
		db.Close()
	}
	return mgr, teardown, &local.Peer
}

func TestClose(t *testing.T) {
	_, teardown, _ := newTest(t, "A1")
	teardown()
}

func TestUnicast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A2")
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

	eventMock.On("newTransactionEvent", &NewTransactionEvent{
		Body: testTransaction.GetBody(),
		Peer: peerA,
	}).Once()

	b, err := proto.Marshal(testTransaction)
	require.NoError(t, err)
	mgrA.Send(b)

	time.Sleep(graceTime)
	eventMock.AssertExpectations(t)
}

func TestBroadcast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A3")
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

	eventMock.On("newTransactionEvent", &NewTransactionEvent{
		Body: testTransaction.GetBody(),
		Peer: peerA,
	}).Twice()

	b, err := proto.Marshal(testTransaction)
	require.NoError(t, err)
	mgrA.Send(b)

	time.Sleep(graceTime)
	eventMock.AssertExpectations(t)
}

func TestDropUnsuccessfulAccept(t *testing.T) {
	mgrA, closeA, _ := newTest(t, "A4")
	defer closeA()
	_, closeB, peerB := newTest(t, "B")
	defer closeB()

	eventMock.On("dropNeighborEvent", &DropNeighborEvent{
		Peer: peerB,
	}).Once()

	err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
	assert.Error(t, err)

	eventMock.AssertExpectations(t)
}

func TestTxRequest(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A5")
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

	eventMock.On("newTransactionEvent", &NewTransactionEvent{
		Body: testTransaction.GetBody(),
		Peer: peerB,
	}).Once()

	b, err := proto.Marshal(&pb.TransactionRequest{Hash: []byte("Hello!")})
	require.NoError(t, err)
	mgrA.RequestTransaction(b)

	time.Sleep(graceTime)
	eventMock.AssertExpectations(t)
}
