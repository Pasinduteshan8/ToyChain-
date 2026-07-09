// Package chain owns the Blockchain type: appending blocks, mining them
// under a proof-of-work target, and (in a later step) validating the chain.
package chain

import (
	"errors"
	"strings"
	"time"

	"toychain/block"
	"toychain/ledger"
)

// Blockchain is an in-memory, append-only sequence of blocks plus the
// proof-of-work difficulty new blocks must satisfy.
type Blockchain struct {
	Blocks       []*block.Block       `json:"blocks"`
	Difficulty   int                  `json:"difficulty"`     // required number of leading hex zeros in a block's hash
	MaxBlockSize int                  `json:"max_block_size"` // max transactions per block; 0 = unlimited (FR-9)
	Pending      []ledger.Transaction `json:"pending"`        // the mempool: transactions waiting to be mined
}

// New creates a blockchain seeded with the deterministic genesis block,
// at the given difficulty (FR-2, FR-5).
func New(difficulty int) *Blockchain {
	return &Blockchain{
		Blocks:     []*block.Block{block.Genesis()},
		Difficulty: difficulty,
	}
}

// Balances replays every transaction in every block, in order, to derive
// current account balances. This is the source of truth for FR-4: balances
// are "derived from the chain," not stored independently. Pending (unmined)
// transactions are NOT included — they aren't real until mined.
func (bc *Blockchain) Balances() *ledger.Balances {
	b := ledger.NewBalances()
	for _, blk := range bc.Blocks {
		for _, tx := range blk.Transactions {
			// Blocks only ever contain transactions that were already
			// validated at submission time (AddTransaction) and at mining
			// time, so Apply should never fail here. We ignore the error
			// deliberately: a failure would indicate a corrupted chain,
			// which chain-level Validate() is responsible for catching,
			// not this replay step.
			_ = b.Apply(tx)
		}
	}
	return b
}

// AddTransaction validates tx against the CURRENT balances (chain state
// plus whatever is already pending) and, if valid, queues it in the
// mempool. This is FR-7's "add a transaction to the pending pool" and is
// where the "reject malformed / overspending transactions" half of FR-4
// actually gets enforced before anything is ever mined.
func (bc *Blockchain) AddTransaction(tx ledger.Transaction) error {
	projected := bc.Balances()
	for _, p := range bc.Pending {
		_ = projected.Apply(p) // fold in already-pending txs so a second
		// tx from the same sender in the same pool can't double-spend
	}
	if err := projected.Validate(tx); err != nil {
		return err
	}
	bc.Pending = append(bc.Pending, tx)
	return nil
}

// Latest returns the most recently added block (the chain tip).
func (bc *Blockchain) Latest() *block.Block {
	return bc.Blocks[len(bc.Blocks)-1]
}

// MiningResult reports what happened during a mining run, used by the CLI
// and by the report's difficulty-vs-effort experiment.
type MiningResult struct {
	Block    *block.Block
	Attempts int // number of hashes tried, i.e. nonce values checked
	Elapsed  time.Duration
}

// MineBlock builds a new block on top of the current tip carrying the given
// transactions, then searches for a nonce whose resulting hash has at least
// bc.Difficulty leading hex zeros (FR-5). It appends the mined block to the
// chain and returns the mining statistics.
func (bc *Blockchain) MineBlock(txs []ledger.Transaction) MiningResult {
	tip := bc.Latest()
	newHeight := tip.Height + 1
	target := strings.Repeat("0", bc.Difficulty)

	start := time.Now()
	b := block.New(newHeight, time.Now().Unix(), txs, tip.Hash)

	attempts := 1 // block.New already computed hash once, at nonce 0
	for !strings.HasPrefix(b.Hash, target) {
		b.Nonce++
		b.Hash = b.CalculateHash()
		attempts++
	}
	elapsed := time.Since(start)

	bc.Blocks = append(bc.Blocks, b)

	return MiningResult{Block: b, Attempts: attempts, Elapsed: elapsed}
}

// MinePending mines a block from the mempool (FR-7's "mine a block from the
// pending pool"). It takes up to MaxBlockSize transactions (all of them, if
// MaxBlockSize is 0/unlimited), mines them into a new block via MineBlock,
// and removes only the mined transactions from Pending. Returns an error if
// there is nothing to mine.
func (bc *Blockchain) MinePending() (MiningResult, error) {
	if len(bc.Pending) == 0 {
		return MiningResult{}, errors.New("no pending transactions to mine")
	}

	n := len(bc.Pending)
	if bc.MaxBlockSize > 0 && n > bc.MaxBlockSize {
		n = bc.MaxBlockSize
	}

	batch := make([]ledger.Transaction, n)
	copy(batch, bc.Pending[:n])

	result := bc.MineBlock(batch)

	remaining := make([]ledger.Transaction, len(bc.Pending)-n)
	copy(remaining, bc.Pending[n:])
	bc.Pending = remaining

	return result, nil
}
