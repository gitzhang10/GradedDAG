package gradeddag

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"time"
)

func genMsgHashSum(data []byte) ([]byte, error) {
	msgHash := sha256.New()
	_, err := msgHash.Write(data)
	if err != nil {
		return nil, err
	}
	return msgHash.Sum(nil), nil
}

// encode encodes the data into bytes.
// Data can be of any type.
// Examples can be seen form the tests.
func encode(data interface{}) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decode decodes bytes into the data.
// Data should be passed in the format of a pointer to a type.
// Examples can be seen form the tests.
func decode(s []byte, data interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(s))
	if err := dec.Decode(data); err != nil {
		return err
	}
	return nil
}

func (b *Block) getHash() ([]byte, error) {
	encodedBlock, err := encode(b)
	if err != nil {
		return nil, err
	}
	return genMsgHashSum(encodedBlock)
}

func (b *Block) getHashAsString() (string, error) {
	hash, err := b.getHash()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

// generate a transaction with s bytes
func generateTX(s int) []byte {
	var trans []byte
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < s; i++ {
		trans = append(trans, byte(rand.Intn(200)))
	}
	return trans
}
