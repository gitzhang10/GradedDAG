package conn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"time"
)

var (
	// ErrTransportShutdown is returned when operations on a transport are
	// invoked after it's been terminated.
	ErrTransportShutdown = errors.New("transport shutdown")
)

// MsgWithSig encapsulates the original msg with the ED25519 signature.
type MsgWithSig struct {
	Msg interface{}
	Sig []byte
}

/*
NetworkTransport provides a network based transport that can be
used to communicate with the remote nodes. It requires
an underlying stream layer to provide a stream abstraction, which can
be simple TCP, TLS, etc.

This transport is very simple and lightweight. Each SendMsg request is
framed by sending a byte that indicates the message type, followed
by the Msg data and signature data.
*/
type NetworkTransport struct {
	connPool     map[string][]*NetConn
	connPoolLock sync.Mutex
	maxPool      int

	msgCh chan MsgWithSig // msgCh is used to transfer data between NetworkTransport and outer variable (e.g., Node)

	reflectedTypesMap map[uint8]reflect.Type

	logger hclog.Logger

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	stream StreamLayer

	// streamCtx is used to cancel existing connection handlers.
	streamCtx     context.Context
	streamCancel  context.CancelFunc
	streamCtxLock sync.RWMutex

	timeout time.Duration
}

// ConnPool returns the connPool field of the NetworkTransport.
func (n *NetworkTransport) ConnPool() map[string][]*NetConn {
	return n.connPool
}

// MsgChan returns the msgCh field of the NetworkTransport.
func (n *NetworkTransport) MsgChan() chan MsgWithSig {
	return n.msgCh
}

// setupStreamContext is used to create a new stream context. This should be
// called with the stream lock held.
func (n *NetworkTransport) setupStreamContext() {
	ctx, cancel := context.WithCancel(context.Background())
	n.streamCtx = ctx
	n.streamCancel = cancel
}

// getStreamContext is used retrieve the current stream context.
func (n *NetworkTransport) getStreamContext() context.Context {
	n.streamCtxLock.RLock()
	defer n.streamCtxLock.RUnlock()
	return n.streamCtx
}

// GetStreamContext is used retrieve the current stream context.
func (n *NetworkTransport) GetStreamContext() context.Context {
	return n.getStreamContext()
}

// listen is used to handling incoming connections.
func (n *NetworkTransport) listen() {
	const baseDelay = 5 * time.Millisecond
	const maxDelay = 1 * time.Second

	var loopDelay time.Duration
	for {
		// Accept incoming connections
		conn, err := n.stream.Accept()
		if err != nil {
			if loopDelay == 0 {
				loopDelay = baseDelay
			} else {
				loopDelay *= 2
			}

			if loopDelay > maxDelay {
				loopDelay = maxDelay
			}

			if !n.IsShutdown() {
				n.logger.Error("failed to accept connection", "error", err)
				return
			}

			select {
			case <-n.shutdownCh:
				return
			case <-time.After(loopDelay):
				continue
			}
		}
		// No error, reset loop delay
		loopDelay = 0

		n.logger.Debug("accepted connection", "local-address", n.LocalAddr(), "remote-address", conn.RemoteAddr().String())

		// Handle the connection in dedicated routine
		go n.handleConn(n.getStreamContext(), conn)
	}
}

// handleConn is used to handle an inbound connection for its lifespan. The
// handler will exit when the passed context is cancelled or the connection is
// closed.
func (n *NetworkTransport) handleConn(connCtx context.Context, conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	dec := codec.NewDecoder(r, &codec.MsgpackHandle{})

	for {
		select {
		case <-connCtx.Done():
			n.logger.Debug("stream layer is closed")
			return
		default:
		}

		if err := n.handleMsg(r, dec); err != nil {
			if err != io.EOF {
				n.logger.Error("failed to decode incoming command", "error", err)
			}
			return
		}
		if err := w.Flush(); err != nil {
			n.logger.Error("failed to flush response", "error", err)
			return
		}
	}
}

// handleCommand is used to decode and return a single msg.
func (n *NetworkTransport) handleMsg(r *bufio.Reader, dec *codec.Decoder) error {
	// Get the msg type
	rpcType, err := r.ReadByte()
	if err != nil {
		return err
	}

	reflectedType, ok := n.reflectedTypesMap[rpcType]
	if !ok {
		return errors.New(fmt.Sprintf("type of the msg (%d) is unknown", rpcType))
	}
	msgBody := reflect.Zero(reflectedType).Interface()
	if err := dec.Decode(&msgBody); err != nil {
		return err
	}

	var sig []byte
	if err := dec.Decode(&sig); err != nil {
		return err
	}

	msgWithSig := MsgWithSig{
		Msg: msgBody,
		Sig: sig,
	}

	select {
	case n.msgCh <- msgWithSig:
	case <-n.shutdownCh:
		return ErrTransportShutdown
	}
	return nil

}

// LocalAddr implements the Transport interface.
func (n *NetworkTransport) LocalAddr() string {
	return n.stream.Addr().String()
}

// IsShutdown is used to check if the transport is shutdown.
func (n *NetworkTransport) IsShutdown() bool {
	select {
	case <-n.shutdownCh:
		return true
	default:
		return false
	}
}

// Close is used to stop the network transport.
func (n *NetworkTransport) Close() error {
	n.shutdownLock.Lock()
	defer n.shutdownLock.Unlock()

	if !n.shutdown {
		close(n.shutdownCh)
		n.stream.Close()
		n.shutdown = true
	}
	return nil
}

func (n *NetworkTransport) dialConn(target string) (*NetConn, error) {
	// Dial a new connection
	conn, err := n.stream.Dial(target, n.timeout)
	if err != nil {
		return nil, err
	}

	// Wrap the conn
	netC := &NetConn{
		target: target,
		conn:   conn,
		w:      bufio.NewWriter(conn),
	}

	netC.enc = codec.NewEncoder(netC.w, &codec.MsgpackHandle{})

	//Done
	return netC, nil
}

// GetConn returns an idle connection. If there is no one, dial a new connection.
func (n *NetworkTransport) GetConn(target string) (*NetConn, error) {
	n.connPoolLock.Lock()
	defer n.connPoolLock.Unlock()
	// Check for an exiting conn
	netConns, ok := n.connPool[target]
	if ok && len(netConns) > 0 {
		var netC *NetConn
		num := len(netConns)
		netC, netConns[num-1] = netConns[num-1], nil
		n.connPool[target] = netConns[:num-1]
		return netC, nil
	}

	return n.dialConn(target)
}

// ReturnConn returns the connection back to the pool.
// To avoid establishing connections repeatedly, try to maintain the net connection for later reusage.
func (n *NetworkTransport) ReturnConn(netC *NetConn) error {
	n.connPoolLock.Lock()
	defer n.connPoolLock.Unlock()

	key := netC.target
	netConns, _ := n.connPool[key]

	if !n.IsShutdown() && len(netConns) < n.maxPool {
		n.connPool[key] = append(netConns, netC)
		return nil
	} else {
		return netC.Release()
	}
}

// NetworkTransportConfig encapsulates configuration for the network transport layer.
type NetworkTransportConfig struct {
	MaxPool int

	ReflectedTypesMap map[uint8]reflect.Type

	Logger hclog.Logger

	// Dialer
	Stream StreamLayer

	// Timeout is used to apply I/O deadlines. For InstallSnapshot, we multiply
	// the timeout by (SnapshotSize / TimeoutScale).
	Timeout time.Duration
}

// NewNetworkTransportWithConfig creates a new network transport with the given config struct.
func NewNetworkTransportWithConfig(
	config *NetworkTransportConfig,
) *NetworkTransport {
	if config.Logger == nil {
		config.Logger = hclog.New(&hclog.LoggerOptions{
			Name:   "BFT-net",
			Output: hclog.DefaultOutput,
			Level:  hclog.DefaultLevel,
		})
	}
	trans := &NetworkTransport{
		connPool:          make(map[string][]*NetConn),
		maxPool:           config.MaxPool,
		msgCh:             make(chan MsgWithSig, 1),
		reflectedTypesMap: config.ReflectedTypesMap,
		logger:            config.Logger,
		shutdownCh:        make(chan struct{}),
		stream:            config.Stream,
		timeout:           config.Timeout,
	}

	// Create the connection context and then start our listener.
	trans.setupStreamContext()
	go trans.listen()

	return trans
}

// NewNetworkTransport creates a new network transport with the given dialer
// and listener. The maxPool controls how many connections we will pool. The
// timeout is used to apply I/O deadlines. For InstallSnapshot, we multiply
// the timeout by (SnapshotSize / TimeoutScale).
func NewNetworkTransport(
	stream StreamLayer,
	timeout time.Duration,
	logOutput io.Writer,
	maxPool int,
	reflectedTypesMap map[uint8]reflect.Type,
) *NetworkTransport {
	if logOutput == nil {
		logOutput = os.Stderr
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "BFT-net",
		Output: logOutput,
		Level:  hclog.DefaultLevel,
	})
	config := &NetworkTransportConfig{Stream: stream, Timeout: timeout, Logger: logger, MaxPool: maxPool,
		ReflectedTypesMap: reflectedTypesMap}
	return NewNetworkTransportWithConfig(config)
}

// SendMsg is used to encode and send the msg.
func SendMsg(conn *NetConn, rpcType uint8, args interface{}, sig []byte) error {
	// Write the msg type
	if err := conn.w.WriteByte(rpcType); err != nil {
		conn.Release()
		return err
	}

	// Send the msg
	if err := conn.enc.Encode(args); err != nil {
		conn.Release()
		return err
	}

	// Send the ED25519 signature
	if err := conn.enc.Encode(sig); err != nil {
		conn.Release()
		return err
	}

	// Flush
	if err := conn.w.Flush(); err != nil {
		conn.Release()
		return err
	}
	return nil
}
