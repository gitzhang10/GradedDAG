package qcdag

import (
	"github.com/gitzhang10/BFT/conn"
	"github.com/gitzhang10/BFT/sign"
)

func (n *Node) broadcastBlock(round uint64) {
	previousHash := n.selectPreviousBlocks(round-1)
	block := n.NewBlock(round, previousHash)
	n.cbc.BroadcastBlock(block)
}

func (n *Node) broadcastElect(round uint64) {
	data, err := encode(round)
	if err != nil {
		panic(err)
	}
	partialSig := sign.SignTSPartial(n.tsPrivateKey, data)
	elect := Elect{
		Sender:     n.name,
		Round:      round,
		PartialSig: partialSig,
	}
	err = n.broadcast(ElectTag, elect)
	if err != nil {
		panic(err)
	}
}

func (n *Node) broadcastDone(done Done) {
	// var doneQc [][]byte
	// qc := sign.AssembleIntactTSPartial(done.Done, n.tsPublicKey, done.Hash, n.quorumNum, n.nodeNum)
	// doneQc = append(doneQc, qc)
	doneMsg := Done{
		DoneSender:  n.name,
		BlockSender: done.BlockSender,
		Done:        done.Done,
		Hash:        nil,
		Round:       done.Round,
	}
	err := n.broadcast(DoneTag, doneMsg)
	if err != nil {
		panic(err)
	}
}

// send message to all nodes
func (n *Node) broadcast(msgType uint8, msg interface{}) error {
	msgAsBytes, err := encode(msg)
	if err != nil {
		return err
	}
	sig := sign.SignEd25519(n.privateKey, msgAsBytes)
	for addrWithPort := range n.clusterAddrWithPorts {
		netConn, err := n.trans.GetConn(addrWithPort)
		if err != nil {
			return err
		}
		if err = conn.SendMsg(netConn, msgType, msg, sig); err != nil {
			return err
		}

		if err = n.trans.ReturnConn(netConn); err != nil {
			return err
		}
	}
	return nil
}

