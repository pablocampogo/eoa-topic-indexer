package types

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func init() {
	gob.Register(IndexedDataGob{})
}

// IndexedDataJSON is the struct used for marshaling data into JSON format for API responses.
type IndexedDataJSON struct {
	Index           int    `json:"index"`
	L1InfoRoot      string `json:"l1InfoRoot"`
	BlockTime       int    `json:"blockTime"`
	BlockParentHash string `json:"blockParentHash"`
}

// Response is the structure used to represent the response for a page of indexed data in JSON format.
type Response struct {
	Data     []*IndexedDataJSON `json:"data"`
	NextPage string             `json:"nextPage"`
}

// IndexedDataGob is the struct used for internal data representation with binary encoding.
type IndexedDataGob struct {
	L1InfoRoot      [32]byte
	BlockTime       uint64
	BlockParentHash [32]byte
}

// Serialize converts the IndexedDataGob struct to a byte slice using gob encoding.
func (i *IndexedDataGob) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(i)
	return buf.Bytes(), err
}

// Deserialize converts a byte slice into an IndexedDataGob struct using gob decoding.
func Deserialize(data []byte) (*IndexedDataGob, error) {
	var i IndexedDataGob
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&i)
	return &i, err
}

// BytesToInt converts a byte slice (in big endian format) into an integer.
func BytesToInt(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}

// IntToBytes converts an integer to a byte slice (in big endian format).
func IntToBytes(n int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}

// ToJSON converts IndexedDataGob to IndexedDataJSON, including converting byte arrays to hex strings.
func (i *IndexedDataGob) ToJSON(key []byte) *IndexedDataJSON {
	return &IndexedDataJSON{
		Index:           BytesToInt(key),
		L1InfoRoot:      hexutil.Encode(i.L1InfoRoot[:]),
		BlockTime:       int(i.BlockTime),
		BlockParentHash: hexutil.Encode(i.BlockParentHash[:]),
	}
}

// Register represents a key-value pair where the key is the block index and the value is the data
// associated with that block. This structure is used to inform the Persistor to store data on the DB.
type Register struct {
	Key  int
	Data *IndexedDataGob
}

// SyncStruct holds information related to block synchronization and is used to inform
// the synchronizer about newly discovered data. It contains the range of blocks to process
// and the corresponding data that needs to be saved for each block.
type SyncStruct struct {
	StartBlock int
	EndBlock   int
	DataToSave map[int][]*IndexedDataGob
}

// Mode is a custom type representing the operation mode (range or full).
type Mode string

var (
	// ModeRange represents the mode where only a specific range of blocks is processed.
	// It focuses on processing data from a given start block to an end block.
	ModeRange Mode = "range"
	// ModeFull represents the mode where all blocks are processed until a specific block tag is reached.
	// It processes blocks starting from the last saved block and continues until a given block tag.
	// ModeFull also keeps the indexer up to date with the tag as it changes, ensuring it stays synchronized with the latest tag value.
	ModeFull Mode = "full"
)
