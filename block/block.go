// Package block defines the Block type and its deterministic hashing scheme.
package block

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"toychain/ledger"
)

// GenesisPrevHash is the fixed, well-known previous-hash value that marks
// the start of the chain. All-zero, 64 hex chars (32 bytes), matching the
// width of a real SHA-256 digest so genesis "looks like" any other block.
const GenesisPrevHash = "0000000000000000000000000000000000000000000000000000000000000"

// Block is a single link in the chain.
type Block struct {
	Height       int                  `json:"height"`
	Timestamp    int64                `json:"timestamp"`
	Transactions []ledger.Transaction `json:"transactions"`
	PrevHash     string               `json:"prev_hash"`
	Nonce        int                  `json:"nonce"`
	Hash         string               `json:"hash"`
}

// New creates a block with the given fields, computes its hash, and returns it.
// Nonce starts at 0; mining (in the chain package) will increment it and
// recompute the hash until the proof-of-work target is met.
func New(height int, timestamp int64, txs []ledger.Transaction, prevHash string) *Block {
	b := &Block{
		Height:       height,
		Timestamp:    timestamp,
		Transactions: txs,
		PrevHash:     prevHash,
		Nonce:        0,
	}
	b.Hash = b.CalculateHash()
	return b
}

// CalculateHash computes SHA-256 over a stable serialisation of the block's
// fields, in this exact order, EXCLUDING the Hash field itself (since Hash
// is derived from everything else, including it would be circular):
//
//  1. Height    (decimal string)
//  2. Timestamp (decimal string)
//  3. Transactions (JSON array — Go's encoding/json marshals struct fields
//     in the order they're declared, so this is deterministic for a fixed
//     Transaction struct definition)
//  4. PrevHash  (raw string)
//  5. Nonce     (decimal string)
//
// These five pieces are concatenated with a "|" separator (to avoid two
// fields' values accidentally running together and colliding) and hashed
// once with SHA-256. Hashing the same block twice always yields the same
// result because every input is either a primitive or a struct with a
// fixed field order — there is no map, and therefore no non-deterministic
// iteration order, anywhere in the payload.
func (b *Block) CalculateHash() string {
	txBytes, err := json.Marshal(b.Transactions)
	if err != nil {
		// Transactions is a plain slice of a plain struct (strings/int64) —
		// this cannot fail in practice, but we don't silently swallow errors.
		panic(fmt.Sprintf("block: failed to marshal transactions: %v", err))
	}

	payload := strconv.Itoa(b.Height) + "|" +
		strconv.FormatInt(b.Timestamp, 10) + "|" +
		string(txBytes) + "|" +
		b.PrevHash + "|" +
		strconv.Itoa(b.Nonce)

	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// Genesis returns the deterministic first block of the chain (FR-2).
// Height 0, a fixed timestamp (so two runs produce byte-identical genesis
// blocks), no transactions, and PrevHash set to the well-known zero value.
func Genesis() *Block {
	return New(0, 0, []ledger.Transaction{}, GenesisPrevHash)
}
