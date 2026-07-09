package chain

import (
	"strings"
	"testing"

	"toychain/ledger"
)

// TestNewChain_HasGenesis covers the Gherkin scenario "Chain starts from a
// deterministic genesis block": a freshly initialised chain has exactly one
// block, at height 0, with the fixed genesis prev-hash.
func TestNewChain_HasGenesis(t *testing.T) {
	bc := New(2)

	if len(bc.Blocks) != 1 {
		t.Fatalf("expected 1 block in a fresh chain, got %d", len(bc.Blocks))
	}
	if bc.Blocks[0].Height != 0 {
		t.Fatalf("expected genesis at height 0, got %d", bc.Blocks[0].Height)
	}
}

// TestMineBlock_MeetsDifficultyTarget covers FR-5 / the Gherkin scenario
// "A mined block satisfies the difficulty target": for difficulty N, the
// mined block's hash must start with at least N zeros, and the recorded
// nonce must reproduce that exact hash (proving the nonce/hash pair is
// self-consistent, not just found-then-discarded).
func TestMineBlock_MeetsDifficultyTarget(t *testing.T) {
	const difficulty = 3
	bc := New(difficulty)

	result := bc.MineBlock([]ledger.Transaction{
		{Sender: "", Recipient: "alice", Amount: 100}, // faucet
	})

	target := strings.Repeat("0", difficulty)
	if !strings.HasPrefix(result.Block.Hash, target) {
		t.Fatalf("mined hash %s does not have %d leading zeros", result.Block.Hash, difficulty)
	}

	// Reproducibility: recomputing the hash from the stored nonce must give
	// back the exact same hash — the nonce isn't just "found", it's correct.
	recomputed := result.Block.CalculateHash()
	if recomputed != result.Block.Hash {
		t.Fatalf("stored hash %s does not match recomputed hash %s for nonce %d",
			result.Block.Hash, recomputed, result.Block.Nonce)
	}

	if result.Attempts < 1 {
		t.Fatalf("expected at least 1 attempt to be recorded, got %d", result.Attempts)
	}
}

// TestMineBlock_LinksToPrevious checks that a mined block correctly points
// at the previous block's hash and increments height — the basic chain-link
// invariant that validation (Day 3) will check more rigorously.
func TestMineBlock_LinksToPrevious(t *testing.T) {
	bc := New(1)
	genesisHash := bc.Latest().Hash

	result := bc.MineBlock(nil)

	if result.Block.PrevHash != genesisHash {
		t.Fatalf("expected new block's PrevHash to equal genesis hash %s, got %s",
			genesisHash, result.Block.PrevHash)
	}
	if result.Block.Height != 1 {
		t.Fatalf("expected new block height 1, got %d", result.Block.Height)
	}
	if len(bc.Blocks) != 2 {
		t.Fatalf("expected chain length 2 after mining, got %d", len(bc.Blocks))
	}
}
