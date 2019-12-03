package buffstreams

import (
	"errors"
	"sync"
)

// ErrAlreadyOpened represents the error where a caller has tried to open the same
// ip / port address more than once.
var ErrAlreadyOpened = errors.New("A connection to this ip / port is already open.")

// ErrNotOpened represents the error where a caller has tried to use a socket to
// an address that they have not opened yet.
var ErrNotOpened = errors.New("A connection to this ip / port must be opened first.")

// Manager represents the object used to govern interactions between tcp endpoints.
// You can use it to read from and write to streaming or non-streaming TCP connections
// and have it handle packaging data with a header describing the size of the data payload.
// This is to make it easy to work with wire formats like ProtocolBuffers, which require
// a custom-delimeter situation to be sent in a streaming fashion.
type Manager struct {
	connections     map[string]*TCPConn
	listeningSocket *TCPListener
	connectionsLock *sync.RWMutex
}

// NewManager creates a new *Manager based on the provided ManagerConfig
func NewManager() *Manager {
	bm := &Manager{
		connections:     make(map[string]*TCPConn),
		connectionsLock: &sync.RWMutex{},
	}
	return bm
}

func (bm *Manager) AddConnection(id string, conn *TCPConn) {
	bm.connectionsLock.Lock()
	defer bm.connectionsLock.Unlock()
	bm.connections[id] = conn
}

func (bm *Manager) GetConnectionIDs() []string {
	bm.connectionsLock.Lock()
	defer bm.connectionsLock.Unlock()
	connections := make([]string, len(bm.connections))
	var i = 0
	for k := range bm.connections {
		connections[i] = k
		i++
	}
	return connections
}

// StartListening is an asyncrhonous, non-blocking method. It begins listening on the given
// port, and fire off a goroutine for every client connection it receives. That goroutine will
// read the fixed header, then the message payload, and then invoke the povided ListenCallbacl.
// In the event of an transport error, it will disconnect the client. It is the clients responsibility
// to re-connect if needed.
func (bm *Manager) StartListening(cfg TCPListenerConfig) error {
	// Example TCPListenerConfig
	// cfg := TCPListenerConfig{
	//   Address: FormatAddress("", port),
	//   Callback: func([]bytes) error {return nil},
	//   MaxMessageSize: 4096,
	//   EnableLogging: False,
	// }

	btl, err := ListenTCP(cfg)
	if err != nil {
		return err
	}
	bm.listeningSocket = btl
	// By design, TCPManager encourages laziness
	return btl.StartListeningAsync()
}

// CloseListener lets you send a signal to a TCPListener that tells it to
// stop accepting new requests. It will finish any requests in flight.
func (bm *Manager) CloseListener() {
	bm.listeningSocket.Close()
}

// Dial must be called before attempting to write. This is because the TCPWriter
// need certain configuration information, which should be provided upfront. Once
// the connection is open, there should be no need to check on it's status. WriteTo
// will attempt to re-use or rebuild the connection using the existing connection if
// any errors occur on a write. It will return the number of bytes written. While
// the TCPWriter makes every attempt to continue to send bytes until they are all
// written, you should always check to make sure this number matches the bytes you
// attempted to write, due to very exceptional cases.
func (bm *Manager) Dial(cfg *TCPConnConfig) error {
	bm.connectionsLock.Lock()
	defer bm.connectionsLock.Unlock()
	if _, ok := bm.connections[cfg.Address]; ok {
		return ErrAlreadyOpened
	}

	btw, err := DialTCP(cfg)
	if err != nil {
		return err
	}
	bm.connections[cfg.Address] = btw
	return nil
}

// CloseWriter lets you send a signal to a TCPWriter that tells it to
// stop accepting new requests. It will finish any requests in flight.
func (bm *Manager) CloseWriter(address string) error {
	bm.connectionsLock.Lock()
	defer bm.connectionsLock.Unlock()
	if btw, ok := bm.connections[address]; ok {
		return btw.Close()
	}
	// If it wasn't opened, we hit this condition - return error
	return ErrNotOpened
}

// Write allows you to dial to a remote or local TCP endpoint, and send a series of
// bytes as messages. Each array of bytes you pass in will be pre-pended with it's size
// within the size of the pre-defined maximum message size. If the connection isn't open yet,
// WriteTo will open it, and cache it. If for anyreason the connection breaks, it will be disposed
// a. If not all bytes can be written,
// WriteTo will keep trying until the full message is delivered, or the connection is broken.
func (bm *Manager) Write(address string, data []byte) (bytesWritten int, err error) {
	// Get the connection if it's cached
	bm.connectionsLock.RLock()
	btw, ok := bm.connections[address]
	bm.connectionsLock.RUnlock()
	if !ok {
		return 0, ErrNotOpened
	}
	bytesWritten, err = btw.Write(data)
	if err != nil {
		//err = btw.Reopen()
		//if err != nil {
		return 0, ErrNotOpened
		//}
		//bytesWritten, err = btw.Write(data)
	}
	return bytesWritten, err
}
