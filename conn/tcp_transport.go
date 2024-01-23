package conn

import (
	"io"
	"net"
	"reflect"
	"time"
)

// StreamLayer is used with the NetworkTransport to provide
// the low level stream abstraction.
type StreamLayer interface {
	net.Listener

	// Dial is used to create a new outgoing connection
	Dial(address string, timeout time.Duration) (net.Conn, error)
}

// TCPStreamLayer implements StreamLayer interface for plain TCP.
type TCPStreamLayer struct {
	listener *net.TCPListener
}

// Dial implements the StreamLayer interface.
func (t *TCPStreamLayer) Dial(address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp", string(address), timeout)
}

// Accept implements the net.Listener interface.
func (t *TCPStreamLayer) Accept() (c net.Conn, err error) {
	return t.listener.Accept()
}

// Close implements the net.Listener interface.
func (t *TCPStreamLayer) Close() (err error) {
	return t.listener.Close()
}

// Addr implements the net.Listener interface.
func (t *TCPStreamLayer) Addr() net.Addr {
	return t.listener.Addr()
}

func newTCPTransport(bindAddr string,
	transportCreator func(stream StreamLayer) *NetworkTransport) (*NetworkTransport, error) {
	// Try to bind
	list, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, err
	}

	// Create stream
	stream := &TCPStreamLayer{
		listener: list.(*net.TCPListener),
	}

	// Create the network transport
	trans := transportCreator(stream)
	return trans, nil
}

// NewTCPTransport returns a NetworkTransport that is built on top of
// a TCP streaming transport layer.
func NewTCPTransport(
	bindAddr string,
	timeout time.Duration,
	logOutput io.Writer,
	maxPool int,
	reflectedTypesMap map[uint8]reflect.Type,
) (*NetworkTransport, error) {
	return newTCPTransport(bindAddr, func(stream StreamLayer) *NetworkTransport {
		return NewNetworkTransport(stream, timeout, logOutput, maxPool, reflectedTypesMap)
	})
}
