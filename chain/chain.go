// =============================================================================
// chain/chain.go — The Blockchain type: appending blocks, mining under
// Proof-of-Work, managing the mempool, and deriving balances.
//
// CONCEPT: Blockchain = Append-Only Linked List
//   This file manages the chain itself — the ordered sequence of blocks.
//   Once a block is added, it can NEVER be removed or edited. That's the
//   "append-only" rule. The chain is the single source of truth for the
//   entire system: balances, transaction history, everything.
//
// CONCEPT: Proof-of-Work Mining (FR-5)
//   Mining is a mathematical guessing game that forces the computer to do
//   "work" before it's allowed to save a block. The Difficulty setting
//   controls HOW MUCH work: a higher difficulty means the hash must start
//   with MORE leading zeros, which takes exponentially more guesses.
//   This is what prevents spam and (in real blockchains) secures the network.
//
// CONCEPT: The Mempool (Pending Transactions)
//   When a user adds a transaction (add-tx), it doesn't go directly into
//   a block. It goes into the "mempool" (Pending slice) — a waiting room.
//   Transactions sit here until someone runs "mine", at which point a batch
//   is pulled from the mempool, bundled into a new block, and the
//   Proof-of-Work mining begins.
// =============================================================================
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
//
// CONCEPT: The Blockchain struct — The Complete System State
//
//   Blocks       — The chain itself: an ordered slice of block pointers.
//                  Blocks[0] is always the genesis block (FR-2).
//                  Blocks[len-1] is the "tip" (most recent block).
//                  This slice only ever grows; elements are never removed.
//
//   Difficulty   — The Proof-of-Work rule (FR-5, FR-9). It specifies how
//                  many leading hex zeros a block's hash must have to be
//                  accepted. Difficulty 3 means the hash must start with
//                  "000..." (3 zeros). Each additional zero makes mining
//                  roughly 16x harder (since each hex digit has 16 possible
//                  values: 0-f).
//
//   MaxBlockSize — How many transactions can fit in one block (FR-9).
//                  0 means unlimited. This is a configurable parameter
//                  that lets you control block capacity.
//
//   Pending      — The mempool: transactions waiting to be mined. They've
//                  been validated but NOT yet included in any block.
//                  They're "real" only after mining.
type Blockchain struct {
	Blocks       []*block.Block       `json:"blocks"`
	Difficulty   int                  `json:"difficulty"`     // required number of leading hex zeros in a block's hash
	MaxBlockSize int                  `json:"max_block_size"` // max transactions per block; 0 = unlimited (FR-9)
	Pending      []ledger.Transaction `json:"pending"`        // the mempool: transactions waiting to be mined
}

// New creates a blockchain seeded with the deterministic genesis block,
// at the given difficulty (FR-2, FR-5).
//
// CONCEPT: Chain Initialisation
//   A brand-new chain always starts with exactly one block: the genesis block.
//   This is hardcoded — no mining required. The genesis block gives Block 1
//   a PrevHash to point at, anchoring the entire chain.
func New(difficulty int) *Blockchain {
	return &Blockchain{
		Blocks:     []*block.Block{block.Genesis()}, // Start with genesis (FR-2)
		Difficulty: difficulty,                       // Set the PoW difficulty (FR-5, FR-9)
	}
}

// Balances replays every transaction in every block, in order, to derive
// current account balances. This is the source of truth for FR-4: balances
// are "derived from the chain," not stored independently. Pending (unmined)
// transactions are NOT included — they aren't real until mined.
//
// CONCEPT: Balance Derivation (FR-4)
//   We don't store balances separately. Instead, we replay EVERY transaction
//   from the genesis block to the tip, in order, applying each one to a
//   fresh balance sheet. This "replay" approach guarantees that balances
//   are always consistent with the chain. If a block is tampered with,
//   the derived balances will reflect the tampered data — but chain
//   validation (FR-6) would catch the tampering before we trust it.
//
//   Why exclude pending transactions?
//   Pending transactions haven't been mined yet — they're just promises.
//   Only mined transactions (in actual blocks) are considered "real."
//   Until a transaction is in a block, it can still be rejected.
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
// mempool.
//
// CONCEPT: Transaction Validation & Double-Spend Prevention (FR-4)
//   Before a transaction enters the mempool, we project the FULL state:
//   1. Replay all confirmed (mined) transactions to get current balances.
//   2. Apply all ALREADY-PENDING transactions on top of those balances.
//   3. Then validate the new transaction against this projected state.
//
//   Step 2 is critical for preventing double-spending within the mempool.
//   Without it, Alice with 100 coins could add two transactions: "send 80
//   to Bob" and "send 80 to Charlie" — both would pass validation against
//   the confirmed balance of 100, but together they spend 160 (fraud!).
//   By folding in pending transactions first, the second tx sees Alice's
//   projected balance is only 20, and correctly rejects the 80-coin send.
func (bc *Blockchain) AddTransaction(tx ledger.Transaction) error {
	// Start with balances derived from the confirmed (mined) chain state.
	projected := bc.Balances()
	// Apply all already-pending transactions so a second tx from the same
	// sender in the same pool can't double-spend.
	for _, p := range bc.Pending {
		_ = projected.Apply(p)
	}
	// Validate the new transaction against the fully-projected state.
	if err := projected.Validate(tx); err != nil {
		return err
	}
	// Transaction is valid — add it to the mempool (waiting room).
	bc.Pending = append(bc.Pending, tx)
	return nil
}

// Latest returns the most recently added block (the chain tip).
// This is the block that the NEXT mined block will point its PrevHash at.
func (bc *Blockchain) Latest() *block.Block {
	return bc.Blocks[len(bc.Blocks)-1]
}

// MiningResult reports what happened during a mining run, used by the CLI
// and by the report's difficulty-vs-effort experiment.
//
// CONCEPT: Mining Statistics
//   After mining, we want to know: how hard was it? MiningResult captures:
//   - Block:    the successfully mined block
//   - Attempts: how many nonce values were tried (= how many hashes computed)
//   - Elapsed:  wall-clock time the mining took
//
//   These stats are useful for understanding the relationship between
//   Difficulty and computational effort. Higher difficulty = more attempts
//   = more time. This is the fundamental trade-off of Proof-of-Work.
type MiningResult struct {
	Block    *block.Block
	Attempts int // number of hashes tried, i.e. nonce values checked
	Elapsed  time.Duration
}

// MineBlock builds a new block on top of the current tip carrying the given
// transactions, then searches for a nonce whose resulting hash has at least
// bc.Difficulty leading hex zeros (FR-5). It appends the mined block to the
// chain and returns the mining statistics.
//
// CONCEPT: Proof-of-Work Mining Loop (FR-5)
//   This is THE core mining algorithm. Here's what happens step by step:
//
//   1. PREPARE: Create a candidate block with Height = tip.Height + 1,
//      the current timestamp, the given transactions, and PrevHash
//      pointing to the current tip's hash.
//
//   2. BUILD THE TARGET: The target is a string of zeros, e.g., "000" for
//      Difficulty 3. The block's hash must START with this string.
//
//   3. THE GUESSING GAME (the for loop):
//      - Check: does the current hash start with enough zeros?
//      - If NO: increment the Nonce by 1, recalculate the hash, try again.
//      - If YES: we found a valid hash! The loop exits.
//
//      We CANNOT change the transaction data (that would be fraud), so the
//      Nonce is our only "steering wheel" — it's a dummy number whose sole
//      purpose is to change the hash output. Because SHA-256 is essentially
//      random, each nonce produces a completely unpredictable hash.
//      On average, for Difficulty d, we need 16^d attempts (since each hex
//      digit has 16 possible values).
//
//   4. APPEND: The successfully mined block is added to the chain.
//
//   5. RECORD: Time taken is measured with time.Since() for reporting.
func (bc *Blockchain) MineBlock(txs []ledger.Transaction) MiningResult {
	tip := bc.Latest()
	newHeight := tip.Height + 1

	// Build the target string: e.g., Difficulty 3 → target = "000".
	// The mined block's hash must start with this prefix.
	target := strings.Repeat("0", bc.Difficulty)

	// Record when mining started (for elapsed time calculation).
	start := time.Now()

	// Create a candidate block. Its initial hash (at Nonce=0) almost certainly
	// won't meet the difficulty target — the mining loop will fix that.
	b := block.New(newHeight, time.Now().Unix(), txs, tip.Hash)

	// THE PROOF-OF-WORK LOOP (FR-5):
	// This is the "mathematical guessing game." We keep incrementing the
	// Nonce and recalculating the hash until we randomly hit one that
	// starts with enough zeros. There is no shortcut — you MUST try
	// nonces one by one. This is what makes mining computationally expensive
	// and is the foundation of blockchain security in real systems.
	attempts := 1 // block.New already computed hash once, at nonce 0
	for !strings.HasPrefix(b.Hash, target) {
		b.Nonce++                  // Try the next nonce value
		b.Hash = b.CalculateHash() // Recalculate hash with the new nonce
		attempts++
	}
	// Mining complete — we found a valid nonce!
	elapsed := time.Since(start)

	// Append the successfully mined block to the chain.
	// This is the "append-only" part — once added, it can never be removed.
	bc.Blocks = append(bc.Blocks, b)

	return MiningResult{Block: b, Attempts: attempts, Elapsed: elapsed}
}

// MinePending mines a block from the mempool (FR-7's "mine a block from the
// pending pool"). It takes up to MaxBlockSize transactions (all of them, if
// MaxBlockSize is 0/unlimited), mines them into a new block via MineBlock,
// and removes only the mined transactions from Pending.
//
// CONCEPT: Mempool → Block Transition
//   This is the lifecycle of a transaction:
//   1. User runs "add-tx" → transaction enters the Pending mempool.
//   2. User runs "mine"  → MinePending() pulls a batch from the mempool,
//      passes them to MineBlock(), and the Proof-of-Work loop begins.
//   3. Once mined, those transactions move from Pending → a Block.
//   4. The block is appended to the chain → transactions are now "confirmed."
//   5. Only confirmed transactions count toward balances.
//
//   If MaxBlockSize is set (FR-9), only the first N pending transactions
//   are mined per block. The rest stay in the mempool for the next mine.
func (bc *Blockchain) MinePending() (MiningResult, error) {
	if len(bc.Pending) == 0 {
		return MiningResult{}, errors.New("no pending transactions to mine")
	}

	// Determine how many transactions to include in this block.
	n := len(bc.Pending)
	if bc.MaxBlockSize > 0 && n > bc.MaxBlockSize {
		n = bc.MaxBlockSize // Cap at MaxBlockSize (FR-9: configurable parameter)
	}

	// Copy the batch out of Pending (don't modify the slice in-place while mining).
	batch := make([]ledger.Transaction, n)
	copy(batch, bc.Pending[:n])

	// Mine the block with the selected transactions (this runs the PoW loop).
	result := bc.MineBlock(batch)

	// Remove the mined transactions from the mempool.
	// Only the mined batch is removed; any remaining transactions stay pending.
	remaining := make([]ledger.Transaction, len(bc.Pending)-n)
	copy(remaining, bc.Pending[n:])
	bc.Pending = remaining

	return result, nil
}
