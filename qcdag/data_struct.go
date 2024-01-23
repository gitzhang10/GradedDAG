package qcdag

type Block struct {
	Sender        string
	Round         uint64
	PreviousHash  map[string][]byte // at least 2f+1 block in last round, map from sender to hash
	Txs           [][]byte
	TimeStamp     int64
}

// Chain stores blocks which are committed
type Chain struct {
	round         uint64 // the max round of the leader that are committed
	blocks        map[string]*Block //map from hash to the block
}

// vote for blocks
type Vote struct {
	VoteSender    string
	BlockSender   string
	Round         uint64
}

// the vote for cbc-output-blocks in round % 2 = 1
type Ready struct {
	ReadySender   string
	BlockSender   string
	Round         uint64
	Hash          []byte // the block hash
	PartialSig    []byte
}

type Done struct {
	DoneSender    string
	BlockSender   string // the node who send the block corresponding with the done
	Done          [][]byte
	Hash          []byte
	Round         uint64
}

// to elect a leader
type Elect struct {
	Sender        string
	Round         uint64
	PartialSig    []byte
}
