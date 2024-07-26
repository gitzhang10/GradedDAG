package gradeddag

import (
	"crypto/ed25519"
	"sync"

	"github.com/gitzhang10/BFT/conn"
	"github.com/gitzhang10/BFT/sign"
	"go.dedis.ch/kyber/v3/share"
)

// It's a simple CBC.
// But in odd round, we add extra vote for blocks, and output a Done.

type CBC struct {
	name                 string
	clusterAddrWithPorts map[string]uint8
	connPool             *conn.NetworkTransport
	nodeNum              int
	quorumNum            int
	pendingBlocks        map[uint64]map[string]*Block            // map from round to sender to block
	pendingVote          map[uint64]map[string]int               // map from round to block_sender to vote count
	pendingReady         map[uint64]map[string]map[string][]byte // map from round to block_sender to ready_sender to parSig
	privateKey           ed25519.PrivateKey
	tsPublicKey          *share.PubPoly
	tsPrivateKey         *share.PriShare
	lock                 sync.RWMutex
	blockCh              chan Block
	doneCh               chan Done
	blockOutput          map[uint64]map[string]bool // mark whether a block has been output before
	doneOutput           map[uint64]map[string]bool // mark whether a done has been output before
	blockSend            map[uint64]bool            // mark whether have sent block in a round
}

func (c *CBC) ReturnBlockChan() chan Block {
	return c.blockCh
}

func (c *CBC) ReturnDoneChan() chan Done {
	return c.doneCh
}

func NewCBCer(name string, clusterAddrWithPorts map[string]uint8, connPool *conn.NetworkTransport, q, n int,
	privateKey ed25519.PrivateKey, tsPublicKey *share.PubPoly, tsPrivateKey *share.PriShare) *CBC {
	return &CBC{
		name:                 name,
		clusterAddrWithPorts: clusterAddrWithPorts,
		connPool:             connPool,
		nodeNum:              n,
		quorumNum:            q,
		pendingBlocks:        make(map[uint64]map[string]*Block),
		pendingVote:          make(map[uint64]map[string]int),
		pendingReady:         make(map[uint64]map[string]map[string][]byte),
		privateKey:           privateKey,
		blockCh:              make(chan Block),
		doneCh:               make(chan Done),
		blockOutput:          make(map[uint64]map[string]bool),
		doneOutput:           make(map[uint64]map[string]bool),
		tsPublicKey:          tsPublicKey,
		tsPrivateKey:         tsPrivateKey,
		blockSend:            make(map[uint64]bool),
	}
}

func (c *CBC) BroadcastBlock(block *Block) {
	err := c.broadcast(ProposalTag, block)
	if err != nil {
		panic(err)
	}
	c.lock.Lock()
	c.blockSend[block.Round] = true
	c.lock.Unlock()
}

func (c *CBC) BroadcastVote(blockSender string, round uint64) {
	vote := Vote{
		VoteSender:  c.name,
		BlockSender: blockSender,
		Round:       round,
	}
	err := c.broadcast(VoteTag, vote)
	if err != nil {
		panic(err)
	}
}

func (c *CBC) broadcastReady(round uint64, hash []byte, blockSender string) {
	partialSig := sign.SignTSPartial(c.tsPrivateKey, hash)
	ready := Ready{
		ReadySender: c.name,
		BlockSender: blockSender,
		Round:       round,
		Hash:        hash,
		PartialSig:  partialSig,
	}
	err := c.broadcast(ReadyTag, ready)
	if err != nil {
		panic(err)
	}
}

func (c *CBC) HandleBlockMsg(block *Block) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.storeBlockMsg(block)
	go c.BroadcastVote(block.Sender, block.Round)
	go c.checkIfQuorumVote(block.Round, block.Sender)
}

func (c *CBC) HandleVoteMsg(vote *Vote) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.storeVoteMsg(vote)
	go c.checkIfQuorumVote(vote.Round, vote.BlockSender)
}

func (c *CBC) handleReadyMsg(ready *Ready) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.storeReadyMsg(ready)
	go c.checkIfQuorumReady(ready)
}

func (c *CBC) storeBlockMsg(block *Block) {
	if _, ok := c.pendingBlocks[block.Round]; !ok {
		c.pendingBlocks[block.Round] = make(map[string]*Block)
	}
	c.pendingBlocks[block.Round][block.Sender] = block
}

func (c *CBC) storeVoteMsg(vote *Vote) {
	if _, ok := c.pendingVote[vote.Round]; !ok {
		c.pendingVote[vote.Round] = make(map[string]int)
	}
	c.pendingVote[vote.Round][vote.BlockSender]++
}

func (c *CBC) storeReadyMsg(ready *Ready) {
	if _, ok := c.pendingReady[ready.Round]; !ok {
		c.pendingReady[ready.Round] = make(map[string]map[string][]byte)
	}
	if _, ok := c.pendingReady[ready.Round][ready.BlockSender]; !ok {
		c.pendingReady[ready.Round][ready.BlockSender] = make(map[string][]byte)
	}
	c.pendingReady[ready.Round][ready.BlockSender][ready.ReadySender] = ready.PartialSig
}

func (c *CBC) checkIfQuorumVote(round uint64, blockSender string) {
	c.lock.Lock()
	voteCount := c.pendingVote[round][blockSender]
	c.lock.Unlock()
	if voteCount >= c.quorumNum {
		go c.tryToOutputBlocks(round, blockSender)
	}
}

func (c *CBC) checkIfQuorumReady(ready *Ready) {
	c.lock.Lock()
	readies, _ := c.pendingReady[ready.Round][ready.BlockSender]
	if _, ok := c.doneOutput[ready.Round]; !ok {
		c.doneOutput[ready.Round] = make(map[string]bool)
	}
	if len(readies) >= c.quorumNum && !c.doneOutput[ready.Round][ready.BlockSender] {
		c.doneOutput[ready.Round][ready.BlockSender] = true
		var partialSig [][]byte
		for _, parSig := range readies {
			partialSig = append(partialSig, parSig)
		}
		c.lock.Unlock()
		// done := sign.AssembleIntactTSPartial(partialSig, c.tsPublicKey, ready.Hash, c.quorumNum, c.nodeNum)
		doneMsg := &Done{
			DoneSender:  c.name,
			BlockSender: ready.BlockSender,
			Done:        partialSig,
			Hash:        ready.Hash,
			Round:       ready.Round,
		}
		c.doneCh <- *doneMsg
	} else {
		c.lock.Unlock()
	}

}

func (c *CBC) tryToOutputBlocks(round uint64, sender string) {
	c.lock.Lock()
	if _, ok := c.blockOutput[round]; !ok {
		c.blockOutput[round] = make(map[string]bool)
	}
	if c.blockOutput[round][sender] {
		c.lock.Unlock()
		return
	}
	if _, ok := c.pendingBlocks[round][sender]; !ok {
		c.lock.Unlock()
		return
	}
	block := c.pendingBlocks[round][sender]
	c.blockOutput[round][sender] = true
	// only send ready for odd blocks
	// we will not send ready for slow blocks
	if block.Round%2 == 1 && !c.blockSend[block.Round+1] {
		c.lock.Unlock()
		hash, _ := block.getHash()
		go c.broadcastReady(block.Round, hash, block.Sender)
	} else {
		c.lock.Unlock()
	}
	c.blockCh <- *block
}

// send message to all nodes
func (c *CBC) broadcast(msgType uint8, msg interface{}) error {
	msgAsBytes, err := encode(msg)
	if err != nil {
		return err
	}
	sig := sign.SignEd25519(c.privateKey, msgAsBytes)
	for addrWithPort := range c.clusterAddrWithPorts {
		netConn, err := c.connPool.GetConn(addrWithPort)
		if err != nil {
			return err
		}
		if err = conn.SendMsg(netConn, msgType, msg, sig); err != nil {
			return err
		}

		if err = c.connPool.ReturnConn(netConn); err != nil {
			return err
		}
	}
	return nil
}
