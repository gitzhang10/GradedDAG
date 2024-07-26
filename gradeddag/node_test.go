package gradeddag

import (
	"crypto/ed25519"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gitzhang10/BFT/config"
	"github.com/gitzhang10/BFT/sign"
)

var clusterAddr = map[string]string{
	"node0": "127.0.0.1",
	"node1": "127.0.0.1",
	"node2": "127.0.0.1",
	"node3": "127.0.0.1",
}
var clusterPort = map[string]int{
	"node0": 8000,
	"node1": 8010,
	"node2": 8020,
	"node3": 8030,
}

func setupNodes(logLevel int, batchSize int, round int) []*Node {
	names := make([]string, 4)
	clusterAddrWithPorts := make(map[string]uint8)
	for name, addr := range clusterAddr {
		rn := []rune(name)
		i, _ := strconv.Atoi(string(rn[4:]))
		names[i] = name
		clusterAddrWithPorts[addr+":"+strconv.Itoa(clusterPort[name])] = uint8(i)
	}

	// create the ED25519 keys
	privKeys := make([]ed25519.PrivateKey, 4)
	pubKeys := make([]ed25519.PublicKey, 4)
	for i := 0; i < 4; i++ {
		privKeys[i], pubKeys[i] = sign.GenED25519Keys()
	}
	pubKeyMap := make(map[string]ed25519.PublicKey)
	for i := 0; i < 4; i++ {
		pubKeyMap[names[i]] = pubKeys[i]
	}

	// create the threshold keys
	shares, pubPoly := sign.GenTSKeys(3, 4)

	// create configs and nodes
	confs := make([]*config.Config, 4)
	nodes := make([]*Node, 4)
	for i := 0; i < 4; i++ {
		confs[i] = config.New(names[i], 10, clusterAddr, clusterPort, nil, clusterAddrWithPorts, nil, pubKeyMap, privKeys[i], pubPoly, shares[i], logLevel, false, batchSize, round)
		nodes[i] = NewNode(confs[i])
		if err := nodes[i].StartP2PListen(); err != nil {
			panic(err)
		}
		nodes[i].InitCBC(confs[i])
	}
	for i := 0; i < 4; i++ {
		go nodes[i].EstablishP2PConns()
	}
	time.Sleep(time.Second)
	return nodes
}

func clean(nodes []*Node) {
	for _, n := range nodes {
		n.trans.GetStreamContext().Done()
		_ = n.trans.Close()
	}
}

func TestWith4Nodes(t *testing.T) {
	nodes := setupNodes(3, 50, 500)
	nodes[0].isFaulty = true
	for i := 0; i < 4; i++ {
		fmt.Printf("node%d starts the GradedDAG!\n", i)
		go nodes[i].RunLoop()
		go nodes[i].HandleMsgLoop()
		go nodes[i].CBCOutputBlockLoop()
		go nodes[i].DoneOutputLoop()
	}

	// wait all nodes finish
	time.Sleep(15 * time.Second)

	clean(nodes)
}
