// =============================================================================
// ledger/transaction.go — The smallest unit of value transfer (FR-4).
//
// CONCEPT: Transactions & Ledger
//   In a real bank, a transaction is: "Alice sends $50 to Bob." In our
//   blockchain it's the same idea, stored as a Go struct with three fields.
//   Transactions are bundled into Blocks (like entries on a ledger page),
//   and the blockchain is the full stack of those pages.
//
// FR-4 (Transactions & Ledger):
//   This struct is the data model. Validation logic (rejecting negatives,
//   checking balances) lives in balances.go. The chain never stores a
//   Transaction it hasn't already validated, so once it's in a block,
//   it's considered final and trustworthy.
// =============================================================================
package ledger

// Transaction is the smallest unit of value transfer in the toy chain.
//
// CONCEPT: What each field represents:
//
//   Sender    — Who is sending money. If this is EMPTY (""), it means this is
//               a "coinbase" or "faucet" transaction — money is being created
//               out of thin air. In real blockchains like Bitcoin, miners get
//               a coinbase reward for finding a valid block. Here, the faucet
//               lets us inject money into the system for testing (FR-4).
//
//   Recipient — Who is receiving money. This can never be empty.
//
//   Amount    — How much is being transferred. Must always be positive (> 0).
//               We use int64 (not float64) to avoid floating-point rounding
//               errors — blockchains deal in exact integer amounts.
//
// CONCEPT: JSON Tags & Persistence (FR-8)
//   The `json:"..."` struct tags tell Go's encoding/json package exactly what
//   field names to use when serialising to/from JSON. This is critical because
//   the chain is saved to chain.json (FR-8), and when we load it back, these
//   tags ensure every field maps to the correct JSON key. Without them, Go
//   would use the capitalised field name ("Sender" instead of "sender"),
//   which would break compatibility if any external tool reads the JSON.
type Transaction struct {
	Sender    string `json:"sender"`    // Who sends — empty string = faucet/coinbase (mints new money)
	Recipient string `json:"recipient"` // Who receives — must always be non-empty
	Amount    int64  `json:"amount"`    // How much — must be positive; int64 avoids floating-point errors
}