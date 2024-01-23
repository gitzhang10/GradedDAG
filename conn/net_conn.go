/*
Package conn implements the connection between a pair of nodes.
The connection will only be used in an unidirectional manner.
For example, if a node (node1) connects to another node (node2, with a port open for listening),
the connection will only be used to send data from node1 to node2.
Also, to make the connection more usable, the connection is encapsulated with the writer and encoder.
*/
package conn

import (
	"bufio"
	"github.com/hashicorp/go-msgpack/codec"
	"net"
)

// NetConn represents a connection established from one node to another.
type NetConn struct {
	target string
	conn   net.Conn
	w      *bufio.Writer
	enc    *codec.Encoder
}

// Release closes the connection in a NetConn variable.
func (n *NetConn) Release() error {
	return n.conn.Close()
}
