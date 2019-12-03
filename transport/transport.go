package transport

import (
	"log"
	"net"
	"sync"
)

// TransportConn wraps a TCPConn my un-/marshaling the packets using protobuf.
type TransportTCP struct {
	cfg             TCPConfig
	connections     map[string]*net.TCPConn
	lock            sync.RWMutex
	listener        *net.TCPListener
	receive         chan *transfer
	shutdownChannel chan struct{}
}

type transfer struct {
	from string
	data []byte
}

type TCPConfig struct {
	// Controls how large the largest Message may be. The server will reject any messages whose clients
	// header size does not match this configuration
	MaxMessageSize int
	// Controls the ability to enable logging errors occurring in the library
	EnableLogging bool
	// The local address to listen for incoming connections on.
	Address string
	// Receive buffer
	Buffer int
}

// NewTransportTCP creates a new transport layer by using the underlying TCPConn.
func NewTransportTCP(cfg TCPConfig) (*TransportTCP, error) {
	t := &TransportTCP{
		connections:     make(map[string]*net.TCPConn),
		receive:         make(chan *transfer, 100),
		cfg:             cfg,
		shutdownChannel: make(chan struct{}),
	}
	if cfg.Buffer > 0 {
		t.receive = make(chan *transfer, cfg.Buffer)
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.cfg.Address)
	if err != nil {
		return nil, err
	}
	receiveSocket, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}
	t.listener = receiveSocket
	return t, nil
}

// ReadFrom implements the Transport ReadFrom method.
func (t *TransportTCP) ReadFrom() ([]byte, string, error) {
	b := make([]byte, t.cfg.MaxMessageSize)
	select {
	case rcv := <-t.receive:
		n := copy(b, rcv.data)
		return b[:n], rcv.from, nil
	default:
		return nil, "", nil
	}
}

// TODO: refactor and add new addConnection / addPeer
// WriteTo implements the Transport WriteTo method.
func (t *TransportTCP) WriteTo(pkt []byte, address string) error {
	t.lock.RLock()
	// if the connection is not yet established, dial first
	if _, ok := t.connections[address]; !ok {
		t.lock.RUnlock()
		err := t.connect(address)
		if err != nil {
			return err
		}
	} else {
		t.lock.RUnlock()
	}

	t.lock.RLock()
	defer t.lock.RUnlock()
	_, err := t.connections[address].Write(pkt)
	return err
}

// Close represents a way to signal to the Listener that it should no longer accept
// incoming connections, and begin to shutdown.
func (t *TransportTCP) Close() {
	close(t.shutdownChannel)
	t.lock.Lock()
	defer t.lock.Unlock()
	for k, c := range t.connections {
		c.Close()
		delete(t.connections, k)
	}
}

// Close represents a way to signal to the Listener that it should no longer accept
// incoming connections, and begin to shutdown.
func (t *TransportTCP) CloseConn(address string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	// if the connection is not yet established, dial first
	if _, ok := t.connections[address]; ok {
		t.connections[address].Close()
		delete(t.connections, address)
	}
}

// LocalAddr returns the local network address.
func (t *TransportTCP) LocalAddr() net.Addr {
	return t.listener.Addr()
}

// connect opens a connection and starts a read loop on the connection
func (t *TransportTCP) connect(address string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return err
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.connections[address] = conn
	go t.readLoop(conn)
	return err
}

// StartListening represents a way to start accepting TCP connections as blocking
func (t *TransportTCP) StartListening() error {
	return t.blockListen()
}

// StartListeningAsync represents a way to start accepting TCP connections as non-blocking
func (t *TransportTCP) StartListeningAsync() error {
	var err error
	go func() {
		err = t.blockListen()
	}()
	return err
}

// TODO: filter allowed addresses already from here
// Actually blocks the thread it's running on, and begins handling incoming
// requests
func (t *TransportTCP) blockListen() error {
	for {
		// Wait for someone to connect
		c, err := t.listener.AcceptTCP()
		if err != nil {
			if t.cfg.EnableLogging {
				log.Printf("Error attempting to accept connection: %s", err)
			}
			select {
			case <-t.shutdownChannel:
				return nil
			default:
				// Nothing, continue to the top of the loop
			}
		} else {
			// Hand this off and immediately listen for more
			t.lock.Lock()
			t.connections[c.RemoteAddr().String()] = c
			t.lock.Unlock()
			go t.readLoop(c)
		}
	}
}

// Handles incoming data over a given connection.
func (t *TransportTCP) readLoop(conn *net.TCPConn) {
	// dataBuffer will hold the message from each read
	dataBuffer := make([]byte, t.cfg.MaxMessageSize)

	// Begin the read loop
	// If there is any error, close the connection and break out of the read-loop.
	for {
		msgLen, err := conn.Read(dataBuffer)
		// TODO: check if getting temporary errors (see server of autopeering)
		if err != nil {
			if t.cfg.EnableLogging {
				log.Printf("Address %s: Failure to read from connection. Underlying error: %s", conn.RemoteAddr(), err)
			}
			t.CloseConn(conn.RemoteAddr().String())
			return
		}
		// We send the received data to the receive channel
		select {
		case t.receive <- &transfer{
			from: conn.RemoteAddr().String(),
			data: dataBuffer[:msgLen],
		}:
			continue
		case <-t.shutdownChannel:
			t.CloseConn(conn.RemoteAddr().String())
			return
		}
	}
}
