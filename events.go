package gossip

import (
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events contains all the events that are triggered during the gossip protocol.
var Events = struct {
	// A NewTransaction event is triggered when a new transaction is received by the gossip protocol.
	NewTransaction *events.Event
	// A RequestTransaction event is triggered when the given transaction is required for the solidification.
	RequestTransaction *events.Event
	// A DropNeighbor event is triggered when a neighbor has been dropped
	DropNeighbor *events.Event
}{
	NewTransaction:     events.NewEvent(newTransaction),
	RequestTransaction: events.NewEvent(requestTransaction),
	DropNeighbor:       events.NewEvent(dropNeighbor),
}

type NewTransactionEvent struct {
	Body []byte
	Peer *peer.Peer
}

type RequestTransactionEvent struct {
	TxHash []byte
}
type DropNeighborEvent struct {
	Peer *peer.Peer
}

func newTransaction(handler interface{}, params ...interface{}) {
	handler.(func(*NewTransactionEvent))(params[0].(*NewTransactionEvent))
}

func requestTransaction(handler interface{}, params ...interface{}) {
	handler.(func(*RequestTransactionEvent))(params[0].(*RequestTransactionEvent))
}

func dropNeighbor(handler interface{}, params ...interface{}) {
	handler.(func(*DropNeighborEvent))(params[0].(*DropNeighborEvent))
}
