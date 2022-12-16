package mindmachine

import (
	"bytes"
	"encoding/gob"
)

//RawMessage STATUS: DEPRECATED (use Nostr events instead)
type RawMessage struct {
	From         Account     `json:"from"`
	Signature    string      `json:"signature"`
	CreatedAt    BlockHeader `json:"created_at_header"` // CreatedAt is the Bitcoin block hash this message was created at
	Time         int64       `json:"unix_time"`
	Instructions interface{} `json:"instructions"`
}

type Account = string

type Wallet struct {
	PrivateKey string
	SeedWords  string
	Account    Account
}

type S256Hash = string
type VotePower = int64

type MindLog struct {
	MindName string
	Comment  string
	Message  interface{}
}

//BlockHeader STATUS: DEPRECATED (use Nostr events instead)
type BlockHeader struct {
	Hash   string `json:"Hash"`
	Time   int64  `json:"Time"`
	Height int64  `json:"Height"`
}

// ToBytes converts any type to a slice of bytes
//todo move this to patches
func ToBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type HashSeq struct {
	Hash      S256Hash
	Sequence  int64
	Mind      string
	Data      bytes.Buffer
	CreatedAt int64
	NailedTo  S256Hash
	EventID   S256Hash //optional
}

//Kind640000 STATUS: DRAFT
//Used for signing state so that we can reach consensus
type Kind640000 struct {
	Mind      string
	Hash      string
	Sequence  int64 `json:"sequence,string"`
	ShareHash string
	Height    int64 `json:"height,string"`
	EventID   string
}
