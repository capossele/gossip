package transport

import (
	"bytes"
	"container/list"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	pb "github.com/iotaledger/autopeering-sim/server/proto"
	"go.uber.org/zap"
)

const (
	// MaxPacketSize specifies the maximum allowed size of packets.
	// Packets larger than this will be cut and thus treated as invalid.
	MaxPacketSize = 1280
)

var (
	ErrTimeout  = errors.New("accept timeout")
	ErrClosed   = errors.New("listener closed")
	ErrNoGossip = errors.New("peer does not have a gossip service")
)

// connection timeouts
const (
	acceptTimeout     = 500 * time.Millisecond
	handshakeTimeout  = 100 * time.Millisecond
	connectionTimeout = acceptTimeout + handshakeTimeout
)

type TransportTCP struct {
	local    *peer.Local
	listener *net.TCPListener
	log      *zap.SugaredLogger

	addAcceptMatcher chan *acceptMatcher
	acceptReceived   chan accept

	closeOnce sync.Once
	wg        sync.WaitGroup
	closing   chan struct{} // if this channel gets closed all pending waits should terminate
}

type acceptMatcher struct {
	peer      *peer.Peer       // connecting peer
	deadline  time.Time        // deadline for the incoming call
	connected chan *connection // a successful connection is signaled here
}

type accept struct {
	fromID peer.ID  // ID of the connecting peer
	req    []byte   // raw data of the handshake request
	conn   net.Conn // the actual network connection
}

func Listen(local *peer.Local, log *zap.SugaredLogger) (*TransportTCP, error) {
	t := &TransportTCP{
		local:            local,
		log:              log,
		addAcceptMatcher: make(chan *acceptMatcher),
		acceptReceived:   make(chan accept),
		closing:          make(chan struct{}),
	}

	gossipAddr := local.Services().Get(service.GossipKey)
	if gossipAddr == nil {
		return nil, ErrNoGossip
	}
	tcpAddr, err := net.ResolveTCPAddr(gossipAddr.Network(), gossipAddr.String())
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP(gossipAddr.Network(), tcpAddr)
	if err != nil {
		return nil, err
	}
	t.listener = listener

	t.wg.Add(2)
	go t.run()
	go t.listenLoop()

	return t, nil
}

// Close stops listening on the gossip address.
func (t *TransportTCP) Close() {
	t.closeOnce.Do(func() {
		close(t.closing)
		if err := t.listener.Close(); err != nil {
			t.log.Warnw("close error", "err", err)
		}
		t.wg.Wait()
	})
}

// LocalAddr returns the listener's network address,
func (t *TransportTCP) LocalAddr() net.Addr {
	return t.listener.Addr()
}

// DialPeer establishes a gossip connection to the given peer.
// If the peer does not accept the connection or the handshake fails, an error is returned.
func (t *TransportTCP) DialPeer(p *peer.Peer) (*connection, error) {
	gossipAddr := p.Services().Get(service.GossipKey)
	if gossipAddr == nil {
		return nil, ErrNoGossip
	}

	conn, err := net.DialTimeout(gossipAddr.Network(), gossipAddr.String(), acceptTimeout)
	if err != nil {
		return nil, err
	}

	err = t.doHandshake(p.PublicKey(), gossipAddr.String(), conn)
	if err != nil {
		return nil, err
	}

	t.log.Debugw("connected", "id", p.ID(), "addr", conn.RemoteAddr(), "direction", "outgoing")
	return newConnection(p, conn), nil
}

// AcceptPeer awaits an incoming connection from the given peer.
// If the peer does not establish the connection or the handshake fails, an error is returned.
func (t *TransportTCP) AcceptPeer(p *peer.Peer) (*connection, error) {
	if p.Services().Get(service.GossipKey) == nil {
		return nil, ErrNoGossip
	}

	connected := make(chan *connection, 1)
	m := &acceptMatcher{
		peer:      p,
		connected: connected,
	}

	// add the matcher
	select {
	case t.addAcceptMatcher <- m:
	case <-t.closing:
		return nil, ErrClosed
	}

	// wait for the connection
	c := <-connected
	if c == nil {
		return nil, ErrTimeout
	}
	t.log.Debugw("connected", "id", p.ID(), "addr", c.conn.RemoteAddr(), "direction", "incoming")
	return c, nil
}

func (t *TransportTCP) closeConnection(c net.Conn) {
	if err := c.Close(); err != nil {
		t.log.Warnw("close error", "err", err)
	}
}

func (t *TransportTCP) run() {
	defer t.wg.Done()

	var (
		mlist   = list.New()
		timeout = time.NewTimer(0)
	)
	defer timeout.Stop()

	<-timeout.C // ignore first timeout

	for {

		// Set the timer so that it fires when the next accept expires
		if el := mlist.Front(); el != nil {
			// the first element always has the closest deadline
			m := el.Value.(*acceptMatcher)
			timeout.Reset(time.Until(m.deadline))
		} else {
			timeout.Stop()
		}

		select {

		// add a new matcher to the list
		case m := <-t.addAcceptMatcher:
			m.deadline = time.Now().Add(connectionTimeout)
			mlist.PushBack(m)

		// on accept received, check all matchers for a fit
		case a := <-t.acceptReceived:
			matched := false
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*acceptMatcher)
				if m.peer.ID() == a.fromID {
					matched = true
					mlist.Remove(el)
					// finish the handshake
					go t.matchAccept(m, a.req, a.conn)
				}
			}
			// close the connection if not matched
			if !matched {
				t.log.Debugw("unexpected connection", "id", a.fromID, "addr", a.conn.RemoteAddr())
				t.closeConnection(a.conn)
			}

		// on timeout, check for expired matchers
		case <-timeout.C:
			now := time.Now()

			// notify and remove any expired matchers
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*acceptMatcher)
				if now.After(m.deadline) || now.Equal(m.deadline) {
					m.connected <- nil
					mlist.Remove(el)
				}
			}

		// on close, notify all the matchers
		case <-t.closing:
			for el := mlist.Front(); el != nil; el = el.Next() {
				el.Value.(*acceptMatcher).connected <- nil
			}
			return

		}
	}
}

func (t *TransportTCP) matchAccept(m *acceptMatcher, req []byte, conn net.Conn) {
	t.wg.Add(1)
	defer t.wg.Done()

	err := t.writeHandshakeResponse(req, conn)
	if err != nil {
		t.log.Warnw("failed handshake", "addr", conn.RemoteAddr(), "err", err)
		m.connected <- nil
		t.closeConnection(conn)
		return
	}
	m.connected <- newConnection(m.peer, conn)
}

func (t *TransportTCP) listenLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.listener.AcceptTCP()
		if err, ok := err.(net.Error); ok && err.Temporary() {
			t.log.Debugw("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			t.log.Warnw("read error", "err", err)
			return
		}

		key, req, err := t.readHandshakeRequest(conn)
		if err != nil {
			t.log.Warnw("failed handshake", "addr", conn.RemoteAddr(), "err", err)
			t.closeConnection(conn)
			continue
		}

		select {
		case t.acceptReceived <- accept{
			fromID: key.ID(),
			req:    req,
			conn:   conn,
		}:
		case <-t.closing:
			t.closeConnection(conn)
			return
		}
	}
}

func (t *TransportTCP) doHandshake(key peer.PublicKey, address string, conn net.Conn) error {
	// TODO: do something better here
	data := []byte(address)
	req := &pb.Packet{
		PublicKey: t.local.PublicKey(),
		Signature: t.local.Sign(data),
		Data:      data,
	}
	b, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	if err := conn.SetWriteDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return err
	}
	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	if err := conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return err
	}
	b = make([]byte, MaxPacketSize)
	n, err := conn.Read(b)
	if err != nil {
		return err
	}

	res := new(pb.Packet)
	if err := proto.Unmarshal(b[:n], res); err != nil {
		return err
	}

	signer, err := peer.RecoverKeyFromSignedData(res)
	if err != nil {
		return err
	}
	if !bytes.Equal(key, signer) {
		return errors.New("invalid key")
	}

	return nil
}

func (t *TransportTCP) readHandshakeRequest(conn net.Conn) (peer.PublicKey, []byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return nil, nil, err
	}
	b := make([]byte, MaxPacketSize)
	n, _ := conn.Read(b)

	req := new(pb.Packet)
	if err := proto.Unmarshal(b[:n], req); err != nil {
		return nil, nil, err
	}

	key, err := peer.RecoverKeyFromSignedData(req)
	if err != nil {
		return nil, nil, err
	}

	return key, req.GetData(), nil
}

func (t *TransportTCP) writeHandshakeResponse(reqData []byte, conn net.Conn) error {
	// TODO: do something meaningful with the request
	data := reqData

	res := &pb.Packet{
		PublicKey: t.local.PublicKey(),
		Signature: t.local.Sign(data),
		Data:      data,
	}

	b, err := proto.Marshal(res)
	if err != nil {
		return err
	}

	if err := conn.SetWriteDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return err
	}
	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	return nil
}
