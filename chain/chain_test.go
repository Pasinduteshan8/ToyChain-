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

/* estMineBlock_MeetsDifficultyTarget
The Goal: Prove that the Proof-of-Work mining algorithm isn't faking it, and actually 
respects the difficulty rules.

How it works:

It sets a strict difficulty = 3 and creates a chain.
It asks the chain to mine a new block containing one faucet transaction.
Check 1 (The Zeros): It looks at the resulting Hash. If it does not start with "000" (three zeros), the test fails.
Check 2 (The Math Proof): It takes the Nonce the miner claims to have found and runs it back through the CalculateHash() function. 
If the recomputed hash doesn't perfectly match the stored hash, it means the mining function is broken or lying.
Check 3 (The Effort): It ensures the loop ran at least once (result.Attempts < 1).

Why it matters: This proves your mining loop works exactly as requested in FR-5.
===================================================================================================
1. The Nonce (The Input)
The Nonce is just a regular counting number. Your computer literally just starts at 1 and counts up.

It tries Nonce 1.

Then Nonce 2.

Then Nonce 300.

Then Nonce 84,592.

It doesn't matter if the Nonce is 1 digit long or 10 digits long. It is just the number your computer is currently guessing.

2. The Hash (The Output & The Difficulty)
Every time your computer guesses a new Nonce, it generates a brand new 64-character Hash. This is where the difficulty is checked.

Let's say your Difficulty is 3. The system says: "I don't care what Nonce you use, but the Hash you give me MUST start with 000."

Here is what the computer's guessing process looks like:

Attempt 1: Nonce is 1 ➔ Hash is 8f4b2a... ❌ (Fails, no zeros)

Attempt 2: Nonce is 2 ➔ Hash is c91b42... ❌ (Fails, no zeros)

Attempt 3: Nonce is 55 ➔ Hash is 0a11fc... ❌ (Fails, only one zero)

Attempt 4: Nonce is 8,923 ➔ Hash is 000b41... ✅ WINNER!

In that example, the winning Nonce was 8,923. The length of that number didn't matter. 
What mattered was that throwing 8,923 into the hashing algorithm magically produced a Hash that started with three zeros (000).
*/
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
