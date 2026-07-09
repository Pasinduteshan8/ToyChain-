package block

import (
	"testing"

	"toychain/ledger"
)

// TestCalculateHash_Deterministic covers FR-3 / the Gherkin scenario
// "Block hashing is deterministic": hashing the same block twice, with
// fixed fields and a fixed nonce, must yield identical results.
func TestCalculateHash_Deterministic(t *testing.T) {
	txs := []ledger.Transaction{
		{Sender: "alice", Recipient: "bob", Amount: 10},
	}
	b := &Block{
		Height:       1,
		Timestamp:    1720000000,
		Transactions: txs,
		PrevHash:     GenesisPrevHash,
		Nonce:        42,
	}

	h1 := b.CalculateHash()
	h2 := b.CalculateHash()

	if h1 != h2 {
		t.Fatalf("hash not deterministic: got %s then %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64 hex chars (SHA-256), got %d: %s", len(h1), h1)
	}
}

// TestCalculateHash_ChangesWithNonce is a sanity check that mining (which
// only varies the nonce) actually has an effect on the hash.
func TestCalculateHash_ChangesWithNonce(t *testing.T) {
	b := &Block{Height: 1, Timestamp: 1, PrevHash: GenesisPrevHash, Nonce: 0}
	h0 := b.CalculateHash()
	b.Nonce = 1
	h1 := b.CalculateHash()

	if h0 == h1 {
		t.Fatalf("expected different hashes for different nonces, got same: %s", h0)
	}
}

// TestGenesis covers FR-2 / the Gherkin scenario "Chain starts from a
// deterministic genesis block": height 0, fixed prev-hash, and running it
// twice produces the same block.
func TestGenesis(t *testing.T) {
	g1 := Genesis()
	g2 := Genesis()

	if g1.Height != 0 {
		t.Fatalf("expected genesis height 0, got %d", g1.Height)
	}
	if g1.PrevHash != GenesisPrevHash {
		t.Fatalf("expected genesis prev-hash %s, got %s", GenesisPrevHash, g1.PrevHash)
	}
	if g1.Hash != g2.Hash {
		t.Fatalf("expected two genesis blocks to be identical, got %s vs %s", g1.Hash, g2.Hash)
	}
}
