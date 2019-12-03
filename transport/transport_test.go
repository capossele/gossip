package transport

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dummyConf(addr string) TCPConfig {
	return TCPConfig{
		MaxMessageSize: 1000,
		EnableLogging:  false,
		Address:        addr,
	}
}
func TestNewTransportTCP(t *testing.T) {
	A, err := NewTransportTCP(dummyConf("127.0.0.1:8080"))
	require.NoError(t, err)
	A.Close()
}

func TestABConnection(t *testing.T) {
	addrA := "127.0.0.1:8081"
	addrB := "127.0.0.1:8082"
	A, err := NewTransportTCP(dummyConf(addrA))
	require.NoError(t, err)
	B, err := NewTransportTCP(dummyConf(addrB))
	require.NoError(t, err)

	err = A.StartListeningAsync()
	require.NoError(t, err)

	err = B.StartListeningAsync()
	require.NoError(t, err)

	msg := "Hello"

	err = A.WriteTo([]byte(msg), addrB)
	require.NoError(t, err)

	var rcv []byte
	var from string
	for {
		rcv, from, err = B.ReadFrom()
		require.NoError(t, err)
		if len(rcv) > 0 {
			break
		}
	}
	assert.Equal(t, []byte(msg), rcv)

	ipA, _, _ := net.SplitHostPort(addrA)
	ipReceived, port, _ := net.SplitHostPort(from)
	assert.Equal(t, ipA, ipReceived)

	err = B.WriteTo([]byte(msg), ipReceived+":"+port)
	require.NoError(t, err)

	for {
		rcv, from, err = A.ReadFrom()
		require.NoError(t, err)
		if len(rcv) > 0 {
			break
		}
	}
	assert.Equal(t, []byte(msg), rcv)

	ipB, _, _ := net.SplitHostPort(addrB)
	ipReceived, _, _ = net.SplitHostPort(from)
	assert.Equal(t, ipB, ipReceived)

	assert.Equal(t, 1, len(A.connections))
	assert.Equal(t, 1, len(B.connections))

	A.Close()
	B.Close()

	assert.Equal(t, 0, len(A.connections))
	assert.Equal(t, 0, len(B.connections))
}
