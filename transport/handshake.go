package transport

import (
	"bytes"
	"time"

	pb "github.com/capossele/gossip/transport/proto"
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/server"
)

const (
	HandshakeExpiration = 20 * time.Second
	VersionNum          = 0
)

// isExpired checks whether the given UNIX time stamp is too far in the past.
func isExpired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= HandshakeExpiration
}

func newHandshakeRequest(fromAddr string, toAddr string) ([]byte, error) {
	m := &pb.HandshakeRequest{
		Version:   VersionNum,
		From:      fromAddr,
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
	return proto.Marshal(m)
}

func newHandshakeResponse(reqData []byte) ([]byte, error) {
	m := &pb.HandshakeResponse{
		ReqHash: server.PacketHash(reqData),
	}
	return proto.Marshal(m)
}

func (t *TransportTCP) validateHandshakeRequest(reqData []byte, fromAddr string) bool {
	m := new(pb.HandshakeRequest)
	if err := proto.Unmarshal(reqData, m); err != nil {
		t.log.Debugw("invalid handshake",
			"err", err,
		)
		return false
	}
	if m.GetVersion() != VersionNum {
		t.log.Debugw("invalid handshake",
			"version", m.GetVersion(),
		)
		return false
	}
	if m.GetFrom() != fromAddr {
		t.log.Debugw("invalid handshake",
			"from", m.GetFrom(),
		)
		return false
	}
	if m.GetTo() != t.LocalAddr().String() {
		t.log.Debugw("invalid handshake",
			"to", m.GetTo(),
		)
		return false
	}
	if isExpired(m.GetTimestamp()) {
		t.log.Debugw("invalid handshake",
			"timestamp", time.Unix(m.GetTimestamp(), 0),
		)
	}

	return true
}

func (t *TransportTCP) validateHandshakeResponse(resData []byte, reqData []byte) bool {
	m := new(pb.HandshakeResponse)
	if err := proto.Unmarshal(resData, m); err != nil {
		t.log.Debugw("invalid handshake",
			"err", err,
		)
		return false
	}
	if !bytes.Equal(m.GetReqHash(), server.PacketHash(reqData)) {
		t.log.Debugw("invalid handshake",
			"hash", m.GetReqHash(),
		)
		return false
	}

	return true
}