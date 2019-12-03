package transport

import (
	"errors"
	"log"
	"net"
	"sync"

	apTransport "github.com/iotaledger/autopeering-sim/transport"
)

// TransportConn wraps a TCPConn my un-/marshaling the packets using protobuf.
type TransportTCP struct {
	cfg             TCPConfig
	connections     map[string]*net.TCPConn
	lock            sync.RWMutex
	socket          *net.TCPListener
	receive         chan Transfer
	shutdownChannel chan struct{}
	shutdownGroup   *sync.WaitGroup
}

type Transfer struct {
	from string
	data []byte
}

type TCPConfig struct {
	// Controls how large the largest Message may be. The server will reject any messages whose clients
	// header size does not match this configuration
	MaxMessageSize int
	// Controls the ability to enable logging errors occurring in the library
	EnableLogging bool
	// The local address to listen for incoming connections on. Typically, you exclude
	// the ip, and just provide port, ie: ":5031"
	Address string
	// The callback to invoke once a full set of message bytes has been received. It
	// is your responsibility to handle parsing the incoming message and handling errors
	// inside the callback
	Callback ListenCallback
}

type ListenCallback func([]byte) error

// NewTransportTCP creates a new transport layer by using the underlying TCPConn.
func NewTransportTCP(cfg TCPConfig) (*TransportTCP, error) {
	t := &TransportTCP{
		connections:     make(map[string]*net.TCPConn),
		receive:         make(chan Transfer, 100),
		cfg:             cfg,
		shutdownChannel: make(chan struct{}),
		shutdownGroup:   &sync.WaitGroup{},
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.cfg.Address)
	if err != nil {
		return nil, err
	}
	receiveSocket, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}
	t.socket = receiveSocket
	return t, nil
}

// ReadFrom implements the Transport ReadFrom method.
func (t *TransportTCP) ReadFrom() ([]byte, string, error) {
	b := make([]byte, apTransport.MaxPacketSize)
	select {
	case rcv := <-t.receive:
		copy(b, rcv.data)
		return b, rcv.from, nil
	default:
		return nil, "", nil
	}
}

// WriteTo implements the Transport WriteTo method.
func (t *TransportTCP) WriteTo(pkt []byte, address string) error {
	var c *net.TCPConn
	var ok bool

	t.lock.RLock()
	defer t.lock.RUnlock()

	if c, ok = t.connections[address]; !ok {
		return errors.New("connection not opened")
	}

	_, err := c.Write(pkt)
	return err
}

// LocalAddr returns the local network address.
func (t *TransportTCP) LocalAddr() net.Addr {
	return t.socket.Addr()
}

// StartListening represents a way to start accepting TCP connections as blocking
func (t *TransportTCP) StartListening() error {
	return t.blockListen()
}

// Close represents a way to signal to the Listener that it should no longer accept
// incoming connections, and begin to shutdown.
func (t *TransportTCP) Close() {
	close(t.shutdownChannel)
	t.shutdownGroup.Wait()
	t.lock.Lock()
	defer t.lock.Unlock()
	for _, c := range t.connections {
		c.Close()
	}
}

// StartListeningAsync represents a way to start accepting TCP connections as non-blocking
func (t *TransportTCP) StartListeningAsync() error {
	var err error
	go func() {
		err = t.blockListen()
	}()
	return err
}

// Actually blocks the thread it's running on, and begins handling incoming
// requests
func (t *TransportTCP) blockListen() error {
	for {
		// Wait for someone to connect
		c, err := t.socket.AcceptTCP()

		// Don't dial out, wrap the underlying conn in one of ours
		//conn.socket = c
		if err != nil {
			if t.cfg.EnableLogging {
				log.Printf("Error attempting to accept connection: %s", err)
			}
			// Stole this approach from http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			// Benefits of a channel for the simplicity of use, but don't have to even check it
			// unless theres an error, so performance impact to incoming conns should be lower
			select {
			case <-t.shutdownChannel:
				return nil
			default:
				// Nothing, continue to the top of the loop
			}
		} else {
			// Hand this off and immediately listen for more
			go t.readLoop(c)
		}
	}
}

// Handles incoming data over a given connection.
func (t *TransportTCP) readLoop(conn *net.TCPConn) {
	// Increment the waitGroup in the event of a shutdown
	t.shutdownGroup.Add(1)
	defer t.shutdownGroup.Done()
	// dataBuffer will hold the message from each read
	dataBuffer := make([]byte, t.cfg.MaxMessageSize)

	// Begin the read loop
	// If there is any error, close the connection and break out of the read-loop.
	for {
		msgLen, err := conn.Read(dataBuffer)
		if err != nil {
			if t.cfg.EnableLogging {
				log.Printf("Address %s: Failure to read from connection. Underlying error: %s", conn.RemoteAddr(), err)
			}
			conn.Close()
			return
		}
		// We send the received data to the receive channel
		select {
		case t.receive <- Transfer{
			from: conn.RemoteAddr().String(),
			data: dataBuffer[:msgLen]}:
			continue
		case <-t.shutdownChannel:
			conn.Close()
			return
		}
	}
}
