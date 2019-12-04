package gossip

import "github.com/iotaledger/hive.go/events"

// Events contains all the events that are triggered during the gossip protocol.
var Events = struct {
	NewTransaction *events.Event
}{
	NewTransaction: events.NewEvent(newTransaction),
}

type NewTransactionEvent struct {
	Body []byte
}

func newTransaction(handler interface{}, params ...interface{}) {
	handler.(func(*NewTransactionEvent))(params[0].(*NewTransactionEvent))
}
