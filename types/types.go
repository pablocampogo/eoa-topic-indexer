package types

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	ErrReqTooBig = errors.New("request was too big")
	ErrMaxRounds = errors.New("max rounds done")
)

func init() {
	gob.Register(IndexedDataGob{})
}

type IndexedDataJSON struct {
	Index           int    `json:"index"`
	L1InfoRoot      string `json:"l1InfoRoot"`
	BlockTime       int    `json:"blockTime"`
	BlockParentHash string `json:"blockParentHash"`
}

type Response struct {
	Data     []*IndexedDataJSON `json:"data"`
	NextPage string             `json:"nextPage"`
}

type IndexedDataGob struct {
	L1InfoRoot      [32]byte
	BlockTime       uint64
	BlockParentHash [32]byte
}

func (i *IndexedDataGob) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(i)
	return buf.Bytes(), err
}

func BytesToInt(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}

func IntToBytes(n int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}

func (i *IndexedDataGob) ToJSON(key []byte) *IndexedDataJSON {
	return &IndexedDataJSON{
		Index:           BytesToInt(key),
		L1InfoRoot:      hexutil.Encode(i.L1InfoRoot[:]),
		BlockTime:       int(i.BlockTime),
		BlockParentHash: hexutil.Encode(i.BlockParentHash[:]),
	}
}

func Deserialize(data []byte) (*IndexedDataGob, error) {
	var i IndexedDataGob
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&i)
	return &i, err
}

type Mode string

var (
	ModeRange Mode = "range"
	ModeFull  Mode = "full"
)

type SyncStruct struct {
	StartBlock int
	EndBlock   int
	DataToSave map[int][]*IndexedDataGob
}

type Register struct {
	Key  int
	Data *IndexedDataGob
}
