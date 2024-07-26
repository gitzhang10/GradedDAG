package gradeddag

import (
	"errors"
	"strconv"
	"time"

	"github.com/gitzhang10/BFT/conn"
)

// start the node to listen for P2P connection
func (n *Node) StartP2PListen() error {
	var err error
	n.trans, err = conn.NewTCPTransport(":"+strconv.Itoa(n.clusterPort[n.name]), 30*time.Second,
		nil, n.maxPool, n.reflectedTypesMap)
	if err != nil {
		return err
	}
	return nil
}

// establish P2P connections with other nodes
func (n *Node) EstablishP2PConns() error {
	if n.trans == nil {
		return errors.New("networkTransport has not been created")
	}
	for addrWithPort := range n.clusterAddrWithPorts {
		connect, err := n.trans.GetConn(addrWithPort)
		if err != nil {
			return err
		}
		err = n.trans.ReturnConn(connect)
		if err != nil {
			return err
		}
		n.logger.Debug("connection has been established", "sender", n.name, "receiver", addrWithPort)
	}
	return nil
}
