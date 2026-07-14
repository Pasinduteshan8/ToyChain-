// =============================================================================
// chain/validate.go — Chain Validation & Tamper Detection (FR-6).
//
// CONCEPT: Chain Validation (The Security Auditor)
//   Validation is the blockchain's immune system. It scans the ENTIRE chain
//   from genesis to tip, checking that nobody edited an old transaction,
//   forged a hash, or broke the chain's structure.
//
//   WHY THIS WORKS: Remember the "avalanche effect" of SHA-256 — change
//   one comma in a block, and its hash changes COMPLETELY. But the NEXT
//   block stores the original hash in its PrevHash field. So tampering
//   with Block 5 causes a mismatch at Block 6, which breaks Block 7, etc.
//   The entire chain unravels from the point of tampering, and validation
//   catches it instantly.
//
// FR-6 (Chain Validation):
//   The Validate() function checks five strict rules on every block:
//   1. Recomputed hash matches the stored hash (catches data tampering).
//   2. PrevHash matches the previous block's actual hash (catches link breaks).
//   3. Hash satisfies the Proof-of-Work difficulty target (catches forged blocks).
//   4. Heights are perfectly sequential (0, 1, 2, 3...).
//   5. Timestamps only move forward (no time travel).
//   It stops at the FIRST failure and reports exactly which block broke.
// =============================================================================
package chain

import (
	"fmt"
	"strings"

	"toychain/block"
)

// ValidationResult reports whether the chain is valid, and if not, exactly
// where and why it first broke (FR-6: "identify the first offending block
// on failure").
//
// CONCEPT: Why we report the FIRST failure only
//   Once a chain link is broken, everything after it is meaningless — a
//   broken link at Block 5 makes Blocks 6, 7, 8... all invalid by
//   extension. So we stop at the first failure and tell the user exactly
//   which block failed and why, rather than flooding them with cascading
//   errors.
type ValidationResult struct {
	Valid        bool
	FailedHeight int    // -1 if Valid
	Reason       string // empty if Valid
}

// Validate walks the chain from genesis to tip and checks four invariants
// on every block (FR-6: Chain Validation / Tamper Detection).
//
// CONCEPT: The Five Checks Explained
//
//   CHECK 1 — Recomputed Hash vs. Stored Hash (TAMPER DETECTION):
//     We take the block's stored fields (Height, Timestamp, Transactions,
//     PrevHash, Nonce) and re-run them through SHA-256. If the result
//     doesn't match the stored Hash, it means SOMETHING in the block was
//     changed AFTER mining. This is the primary tamper-detection mechanism.
//
//     Real-world analogy: It's like re-weighing a sealed package. If the
//     weight doesn't match what's on the label, someone opened it.
//
//   CHECK 2 — PrevHash Link Integrity (CHAIN CONTINUITY):
//     block[i].PrevHash must exactly equal block[i-1].Hash. This is what
//     makes the chain a CHAIN. Even if a tampered block's own stored hash
//     were somehow fixed to look self-consistent, the next block's PrevHash
//     still points at the ORIGINAL hash, catching the tampering.
//
//     Real-world analogy: Each page in a notarised ledger has a reference
//     number to the previous page. If someone rips out page 5 and replaces
//     it, page 6 still references the original page 5's number.
//
//   CHECK 3 — Proof-of-Work Compliance (MINING LEGITIMACY):
//     block[i].Hash must start with at least `Difficulty` leading zeros.
//     This ensures the block was actually mined (not just fabricated with
//     a random hash). Genesis is exempt because it's hardcoded, not mined.
//
//   CHECK 4 — Height & Timestamp Consistency (STRUCTURAL INTEGRITY):
//     Heights must increase by exactly 1 (0, 1, 2, 3...) — no gaps,
//     no duplicates, no going backwards. Timestamps must be non-decreasing
//     (a block can't claim to be older than its predecessor).
func (bc *Blockchain) Validate() ValidationResult {
	// Build the PoW target string: e.g., Difficulty 3 → "000".
	// Every non-genesis block's hash must start with this prefix.
	target := strings.Repeat("0", bc.Difficulty)

	for i, b := range bc.Blocks {
		// =====================================================================
		// CHECK 1: TAMPER DETECTION — Does the stored hash match a fresh one?
		// Re-hash the block from its stored fields. If someone changed ANY
		// field (even one character in a transaction), the recomputed hash
		// will differ from the stored hash, exposing the tampering.
		// =====================================================================
		if b.CalculateHash() != b.Hash {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: stored hash %s does not match recomputed hash %s (block contents were altered after mining)",
					b.Height, b.Hash, b.CalculateHash()),
			}
		}

		if i == 0 {
			// GENESIS BLOCK: Special case — it has no predecessor, so we only
			// check that it carries the well-known all-zeros PrevHash (FR-2).
			// Genesis is exempt from the PoW check because it's hardcoded
			// into the system at startup, not mined.
			if b.PrevHash != block.GenesisPrevHash {
				return ValidationResult{
					Valid: false, FailedHeight: b.Height,
					Reason: fmt.Sprintf("genesis block has non-standard prev-hash %s", b.PrevHash),
				}
			}
			continue
		}

		prev := bc.Blocks[i-1]

		// =====================================================================
		// CHECK 2: CHAIN LINK — Does PrevHash match the previous block's hash?
		// This is the fundamental chain integrity check. If someone tampers
		// with an old block, that block's hash changes, but the next block's
		// PrevHash still holds the ORIGINAL hash — creating a mismatch.
		// This is what makes the blockchain "tamper-evident."
		// =====================================================================
		if b.PrevHash != prev.Hash {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: prev-hash %s does not match previous block's actual hash %s (chain link broken)",
					b.Height, b.PrevHash, prev.Hash),
			}
		}

		// =====================================================================
		// CHECK 3: PROOF-OF-WORK — Does the hash meet the difficulty target?
		// The hash must start with `Difficulty` leading zeros. This proves
		// the miner actually did the computational work to find a valid nonce,
		// and didn't just fabricate a block with a random hash.
		// =====================================================================
		if !strings.HasPrefix(b.Hash, target) {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: hash %s does not satisfy difficulty target of %d leading zeros",
					b.Height, b.Hash, bc.Difficulty),
			}
		}

		// =====================================================================
		// CHECK 4: STRUCTURAL — Are heights sequential and timestamps valid?
		// Heights must be exactly prev+1 (no gaps, no duplicates).
		// Timestamps must be non-decreasing (no time travel).
		// =====================================================================
		if b.Height != prev.Height+1 {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: height does not follow previous block's height %d",
					b.Height, prev.Height),
			}
		}
		if b.Timestamp < prev.Timestamp {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: timestamp %d is earlier than previous block's timestamp %d",
					b.Height, b.Timestamp, prev.Timestamp),
			}
		}
	}

	// All blocks passed all checks — the chain is intact and untampered.
	return ValidationResult{Valid: true, FailedHeight: -1}
}
