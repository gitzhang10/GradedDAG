package qcdag

func (n *Node) HandleMsgLoop() {
	msgCh := n.trans.MsgChan()
	for {
		select {
		case msgWithSig := <-msgCh:
			if n.isFaulty {
				continue
			}
			switch msgAsserted := msgWithSig.Msg.(type) {
			case Block:
				if !n.verifySigED25519(msgAsserted.Sender, msgWithSig.Msg, msgWithSig.Sig) {
					n.logger.Error("fail to verify the block's signature", "round", msgAsserted.Round,
						"sender", msgAsserted.Sender)
					continue
				}
				go n.cbc.HandleBlockMsg(&msgAsserted)
			case Elect:
				if !n.verifySigED25519(msgAsserted.Sender, msgWithSig.Msg, msgWithSig.Sig) {
					n.logger.Error("fail to verify the echo's signature", "round", msgAsserted.Round,
						"sender", msgAsserted.Sender)
					continue
				}
				go n.handleElectMsg(&msgAsserted)
			case Ready:
				if !n.verifySigED25519(msgAsserted.ReadySender, msgWithSig.Msg, msgWithSig.Sig) {
					n.logger.Error("fail to verify the ready's signature", "round", msgAsserted.Round,
						"sender", msgAsserted.ReadySender, "blockSender", msgAsserted.BlockSender)
					continue
				}
				go n.cbc.handleReadyMsg(&msgAsserted)
			case Done:
				if !n.verifySigED25519(msgAsserted.DoneSender, msgWithSig.Msg, msgWithSig.Sig) {
					n.logger.Error("fail to verify the done's signature", "round", msgAsserted.Round,
						"sender", msgAsserted.DoneSender, "blockSender", msgAsserted.BlockSender)
					continue
				}
				go n.handleDoneMsg(&msgAsserted)
			case Vote:
				if !n.verifySigED25519(msgAsserted.VoteSender, msgWithSig.Msg, msgWithSig.Sig) {
					n.logger.Error("fail to verify the vote's signature", "round", msgAsserted.Round,
						"sender", msgAsserted.VoteSender, "blockSender", msgAsserted.BlockSender)
					continue
				}
				go n.cbc.HandleVoteMsg(&msgAsserted)
			}
		}
	}
}

func (n *Node) handleCBCBlock(block *Block) {
	go n.tryToUpdateDAG(block)
}

func (n *Node) handleElectMsg(elect *Elect) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.storeElectMsg(elect)
	n.tryToElectLeader(elect.Round)
}

func (n *Node) handleDoneMsg(done *Done) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.storeDone(done)
	go n.tryToNextRound(done.Round)
	n.tryToCommitLeader(done.Round)
}

func (n *Node) CBCOutputBlockLoop() {
	dataCh := n.cbc.ReturnBlockChan()
	for {
		select {
		case block := <-dataCh:
			n.logger.Debug("Block is received by from CBC", "node", n.name, "round",
				block.Round, "proposer", block.Sender)
			go n.handleCBCBlock(&block)
		}
	}
}

func (n *Node) DoneOutputLoop() {
	dataCh := n.cbc.ReturnDoneChan()
	for {
		select {
		case done := <-dataCh:
			go n.handleDoneMsg(&done)
			// make sure every node can get 2f+1 done
			go n.broadcastDone(done)
		}
	}
}
