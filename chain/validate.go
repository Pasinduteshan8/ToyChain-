package chain

import (
	"fmt"
	"strings"

	"toychain/block"
)

// ValidationResult reports whether the chain is valid, and if not, exactly
// where and why it first broke (FR-6: "identify the first offending block
// on failure").
type ValidationResult struct {
	Valid        bool
	FailedHeight int    // -1 if Valid
	Reason       string // empty if Valid
}

// Validate walks the chain from genesis to tip and checks four invariants
// on every block:
//
//  1. Recomputed hash: rehashing the block's stored fields must reproduce
//     its stored Hash. If it doesn't, SOMETHING in the block was changed
//     after mining — this is what catches tampering (FR-6's tamper-evidence
//     scenario).
//  2. Prev-hash link: block[i].PrevHash must equal block[i-1].Hash. This is
//     what makes tampering with an OLD block visible even if that old
//     block's own stored hash were somehow left self-consistent: the very
//     next block still points at the original, now-orphaned hash.
//  3. Proof-of-work: block[i].Hash must satisfy the difficulty target
//     (skipped for genesis, which is exempt from mining).
//  4. Height/timestamp consistency: heights must increase by exactly 1 from
//     the previous block, and timestamps must be non-decreasing.
//
// Validation stops at the FIRST failing block and reports it — later
// blocks aren't checked, since a broken link early in the chain makes
// everything after it meaningless anyway.
func (bc *Blockchain) Validate() ValidationResult {
	target := strings.Repeat("0", bc.Difficulty)

	for i, b := range bc.Blocks {
		// Check 1: stored hash matches a fresh recomputation.
		if b.CalculateHash() != b.Hash {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: stored hash %s does not match recomputed hash %s (block contents were altered after mining)",
					b.Height, b.Hash, b.CalculateHash()),
			}
		}

		if i == 0 {
			// Genesis: only check it carries the fixed prev-hash value.
			if b.PrevHash != block.GenesisPrevHash {
				return ValidationResult{
					Valid: false, FailedHeight: b.Height,
					Reason: fmt.Sprintf("genesis block has non-standard prev-hash %s", b.PrevHash),
				}
			}
			continue
		}

		prev := bc.Blocks[i-1]

		// Check 2: prev-hash link is intact.
		if b.PrevHash != prev.Hash {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: prev-hash %s does not match previous block's actual hash %s (chain link broken)",
					b.Height, b.PrevHash, prev.Hash),
			}
		}

		// Check 3: proof-of-work target satisfied.
		if !strings.HasPrefix(b.Hash, target) {
			return ValidationResult{
				Valid: false, FailedHeight: b.Height,
				Reason: fmt.Sprintf(
					"block %d: hash %s does not satisfy difficulty target of %d leading zeros",
					b.Height, b.Hash, bc.Difficulty),
			}
		}

		// Check 4: height and timestamp consistency.
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

	return ValidationResult{Valid: true, FailedHeight: -1}
}
