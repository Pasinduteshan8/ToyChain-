// =============================================================================
// block/block.go — The Block type, its deterministic hashing, and genesis (FR-1, FR-2, FR-3).
//
// CONCEPT: Block Structure (FR-1)
//
//	A block is a single "page" in our blockchain ledger. It bundles together
//	a set of transactions with metadata that links it to the previous block.
//	Think of it like a page in a physical ledger where each page is stamped,
//	numbered, and chained to the page before it.
//
// CONCEPT: Blockchain as an Append-Only Linked List
//
//	A blockchain is NOT magic — it is just an append-only linked list.
//	Instead of memory addresses (like a traditional linked list in CS),
//	blocks are linked together using cryptographic hashes. Each block
//	stores the hash of the block before it (PrevHash), so you can only
//	ADD to the end; you can NEVER edit the past without breaking every
//	subsequent link in the chain. This is what makes blockchains tamper-evident.
//
// CONCEPT: SHA-256 Hash (FR-3)
//
//	SHA-256 is a deterministic digital fingerprint. Feed it the exact same
//	data, and it produces the exact same 64-character hex string every time.
//	Change just ONE comma, and the ENTIRE string changes unpredictably.
//	This "avalanche effect" is what makes tampering detectable: if someone
//	changes a transaction in Block 5, Block 5's hash changes, which means
//	Block 6's PrevHash no longer matches, which breaks Block 7's link, etc.
//	The entire chain unravels from the point of tampering.
//
// =============================================================================
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
// the start of the chain.
//
// CONCEPT: Genesis Block (FR-2)
//
//	Because every block points to the one before it via PrevHash, Block 0
//	(the "genesis block") has NOTHING to point to — there is no Block -1.
//	So we hardcode a fake previous hash of all zeros. It's 64 hex characters
//	long (= 32 bytes) to match the width of a real SHA-256 digest, so
//	genesis "looks like" any other block structurally.
//
//	Why all zeros? It's an arbitrary convention used by most blockchains
//	(Bitcoin's genesis block does the same thing). The key requirement is
//	that it's a fixed, deterministic value so that every node in a network
//	would produce the exact same genesis block independently.
const GenesisPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Block is a single link in the chain (FR-1: Block Structure).
//
// CONCEPT: What each field represents:
//
//	Height       — The block's position in the chain (0 = genesis, 1 = first
//	               real block, 2 = second, etc.). Heights must be perfectly
//	               sequential with no gaps — this is checked during
//	               validation (FR-6).
//
//	Timestamp    — When this block was created, as a Unix timestamp (seconds
//	               since Jan 1, 1970). Used during validation to ensure
//	               timestamps only move forward (FR-6). The genesis block
//	               uses 0 so it's deterministic across runs.
//
//	Transactions — The actual data this block carries: a list of value
//	               transfers (e.g., "Alice sends 50 to Bob"). This is
//	               the "payload" of the block. Each transaction was
//	               validated before being included (FR-4).
//
//	PrevHash     — The SHA-256 hash of the PREVIOUS block in the chain.
//	               THIS IS THE LINK. It's what turns a list of blocks
//	               into a "chain." If anyone tampers with a previous
//	               block, its hash changes, and this field won't match
//	               anymore — the chain is broken. (FR-6 checks this.)
//
//	Nonce        — "Number Used Once." This is the "steering wheel" for
//	               mining (FR-5: Proof-of-Work). We can't change the
//	               transaction data (that would be fraud), so we attach
//	               this dummy number and increment it over and over,
//	               completely changing the Hash each time, until we find
//	               a Hash that starts with enough zeros to satisfy the
//	               Difficulty rule.
//
//	Hash         — The SHA-256 fingerprint of this ENTIRE block (computed
//	               from Height + Timestamp + Transactions + PrevHash + Nonce).
//	               Notice: Hash is EXCLUDED from its own computation —
//	               including it would be circular (you'd need the hash
//	               to compute the hash). This is the block's identity.
//
// CONCEPT: JSON Tags & Persistence (FR-8)
//
//	The `json:"..."` tags control how each field is named in the chain.json
//	file. When Save() serialises the chain and Load() reads it back, these
//	tags ensure consistent field mapping.
type Block struct {
	Height       int                  `json:"height"`       // Block position in the chain (0 = genesis)
	Timestamp    int64                `json:"timestamp"`    // Unix timestamp of when the block was created
	Transactions []ledger.Transaction `json:"transactions"` // The value transfers this block carries
	PrevHash     string               `json:"prev_hash"`    // SHA-256 hash of the previous block (THE CHAIN LINK)
	Nonce        int                  `json:"nonce"`        // The Proof-of-Work steering wheel (FR-5)
	Hash         string               `json:"hash"`         // SHA-256 fingerprint of this block's contents
}

// New creates a block with the given fields, computes its hash, and returns it.
// Nonce starts at 0; mining (in the chain package) will increment it and
// recompute the hash until the proof-of-work target is met.
//
// CONCEPT: Block Construction
//
//	When we create a new block, we immediately calculate its hash at Nonce=0.
//	This hash almost certainly WON'T satisfy the Difficulty requirement, so
//	the mining loop in chain.go's MineBlock() will keep bumping the Nonce
//	and recalculating until it finds a valid hash. The block returned here
//	is a "candidate block" — not yet mined, not yet added to the chain.
func New(height int, timestamp int64, txs []ledger.Transaction, prevHash string) *Block {
	b := &Block{
		Height:       height,
		Timestamp:    timestamp,
		Transactions: txs,
		PrevHash:     prevHash,
		Nonce:        0, // Mining will increment this
	}
	b.Hash = b.CalculateHash() // Compute initial hash at Nonce=0
	return b
}

// CalculateHash computes SHA-256 over a stable serialisation of the block's
// fields (FR-3: Deterministic Hashing).
//
// CONCEPT: Why Deterministic Hashing Matters
//
//	The entire security model of a blockchain depends on one property:
//	if you hash the same block data, you ALWAYS get the same hash.
//	This is what allows anyone to VERIFY a block independently — just
//	recalculate the hash and compare. If someone tampered with any field,
//	the recalculated hash won't match the stored one.
//
// CONCEPT: The Hashing Recipe (order matters!)
//
//	We concatenate five fields in this EXACT order, separated by "|":
//
//	  Height | Timestamp | JSON-Transactions | PrevHash | Nonce
//
//	Why this specific order?
//	1. It must be FIXED — if two nodes hash fields in different orders,
//	   they'll get different hashes for the same block, and the chain
//	   becomes inconsistent.
//	2. The "|" separator prevents "field collision" — without it,
//	   Height=12 + Timestamp=345 would look the same as Height=1 +
//	   Timestamp=2345, both producing "12345".
//
// CONCEPT: Why Hash EXCLUDES the Hash field itself
//
//	Notice we hash: Height, Timestamp, Transactions, PrevHash, Nonce.
//	We deliberately EXCLUDE the Hash field. Why? Because Hash is the
//	OUTPUT of this function — if we included it as INPUT, we'd need to
//	know the hash before computing it. That's circular and impossible.
//
// CONCEPT: Why Transactions are JSON-marshalled
//
//	Transactions is a slice of structs. To turn it into a string for
//	hashing, we use json.Marshal(). Go's encoding/json marshals struct
//	fields in the order they're declared (Sender, Recipient, Amount),
//	which is deterministic. We do NOT use a map anywhere in the payload
//	(maps have random iteration order in Go), so the hash is stable.
func (b *Block) CalculateHash() string {
	// Step 1: Convert the Transactions slice to a JSON byte array.
	// This gives us a deterministic string representation of the transaction data.
	txBytes, err := json.Marshal(b.Transactions)
	if err != nil {
		// Transactions is a plain slice of a plain struct (strings/int64) —
		// this cannot fail in practice, but we don't silently swallow errors.
		panic(fmt.Sprintf("block: failed to marshal transactions: %v", err))
	}

	// Step 2: Concatenate all fields in the documented order with "|" separator.
	// This is the "payload" that gets hashed. Every field except Hash is included.
	payload := strconv.Itoa(b.Height) + "|" +
		strconv.FormatInt(b.Timestamp, 10) + "|" +
		string(txBytes) + "|" +
		b.PrevHash + "|" +
		strconv.Itoa(b.Nonce)

	// Step 3: Feed the payload into SHA-256.
	// sha256.Sum256 returns a [32]byte array (256 bits = 32 bytes).
	// hex.EncodeToString converts those 32 bytes into a 64-character hex string
	// (each byte becomes 2 hex chars), e.g., "a3f2b7..." — this is the block's Hash.
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// Genesis returns the deterministic first block of the chain (FR-2).
//
// CONCEPT: The Genesis Block — The Anchor of the Chain
//
//	The genesis block is special:
//	- Height 0: It's the very first block.
//	- Timestamp 0: Fixed at zero so that two independent runs of the program
//	  produce the EXACT same genesis block (byte-identical). If we used
//	  time.Now(), every run would produce a different genesis hash, and
//	  two nodes could never agree on the chain's starting point.
//	- No transactions: The genesis block is structural, not transactional.
//	  It exists only to give Block 1 something to point its PrevHash at.
//	- PrevHash = all zeros: Because there IS no previous block. This is
//	  hardcoded as GenesisPrevHash (64 zero chars) by convention.
//
//	Every blockchain starts with exactly one genesis block. It's the "root"
//	of the append-only linked list. If two blockchains have different genesis
//	blocks, they are completely different chains.
func Genesis() *Block {
	return New(0, 0, []ledger.Transaction{}, GenesisPrevHash)
}
