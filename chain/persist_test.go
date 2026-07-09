package chain

import (
	"os"
	"path/filepath"
	"testing"

	"toychain/ledger"
)

// TestSaveLoad_RoundTrip covers FR-8: a saved chain, reloaded, must
// validate the same way and preserve block data exactly.
func TestSaveLoad_RoundTrip(t *testing.T) {
	bc := New(2)
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "alice", Amount: 100})
	bc.MinePending()

	path := filepath.Join(t.TempDir(), "chain.json")
	if err := bc.Save(path); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := Load(path, 2)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}

	if len(loaded.Blocks) != len(bc.Blocks) {
		t.Fatalf("expected %d blocks after reload, got %d", len(bc.Blocks), len(loaded.Blocks))
	}
	if loaded.Blocks[1].Hash != bc.Blocks[1].Hash {
		t.Fatalf("block hash mismatch after reload: %s vs %s", loaded.Blocks[1].Hash, bc.Blocks[1].Hash)
	}
	if !loaded.Validate().Valid {
		t.Fatal("expected reloaded chain to still validate")
	}
}

// TestLoad_MissingFileReturnsFreshChain covers the "no file yet" path.
func TestLoad_MissingFileReturnsFreshChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	bc, err := Load(path, 3)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if len(bc.Blocks) != 1 {
		t.Fatalf("expected a fresh chain with just genesis, got %d blocks", len(bc.Blocks))
	}
	if bc.Difficulty != 3 {
		t.Fatalf("expected default difficulty 3, got %d", bc.Difficulty)
	}
	_ = os.Remove(path) // no-op, just ensuring no leftover
}
