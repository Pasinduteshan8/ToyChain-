package chain

import (
	"strings"
	"testing"

	"toychain/ledger"
)

// TestValidate_HonestChain covers the Gherkin scenario "An honest chain
// validates successfully": several mined blocks, all links and PoW intact.
func TestValidate_HonestChain(t *testing.T) {
	bc := New(2)
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "alice", Amount: 100})
	bc.MinePending()
	bc.AddTransaction(ledger.Transaction{Sender: "alice", Recipient: "bob", Amount: 30})
	bc.MinePending()

	result := bc.Validate()
	if !result.Valid {
		t.Fatalf("expected honest chain to validate, got invalid: %s", result.Reason)
	}
}

// TestValidate_DetectsTamperedTransaction covers FR-6's tamper-evidence
// scenario: modifying a transaction inside an earlier block must make
// validation fail, and it must correctly identify that block.
func TestValidate_DetectsTamperedTransaction(t *testing.T) {
	bc := New(2)
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "alice", Amount: 100})
	bc.MinePending() // block 1
	bc.AddTransaction(ledger.Transaction{Sender: "alice", Recipient: "bob", Amount: 30})
	bc.MinePending() // block 2

	// Sanity check: chain is valid before tampering.
	if !bc.Validate().Valid {
		t.Fatal("chain should be valid before tampering")
	}

	// Reach into block 1 (the "early block") and silently change the
	// amount, WITHOUT re-mining — this simulates an attacker editing
	// history after the fact rather than a legitimate re-mine.
	bc.Blocks[1].Transactions[0].Amount = 999999

	result := bc.Validate()
	if result.Valid {
		t.Fatal("expected tampered chain to be invalid, but Validate reported it valid")
	}
	if result.FailedHeight != 1 {
		t.Fatalf("expected tampering to be caught at block height 1, got %d", result.FailedHeight)
	}
	if !strings.Contains(result.Reason, "recomputed hash") {
		t.Fatalf("expected reason to cite a hash mismatch, got: %s", result.Reason)
	}
}

// TestValidate_DetectsBrokenPrevHashLink covers the other half of FR-6, and
// the real argument for why proof-of-work + hash-linking matters: an
// attacker who tampers with an old block must also RE-MINE it (find a new
// nonce satisfying the difficulty target) for that block to look internally
// valid on its own. But even after doing that expensive work, the NEXT
// block still points at the block's ORIGINAL hash — so the link check
// catches the tamper one block downstream, and the attacker would have to
// re-mine every subsequent block too, all the way to the tip, to hide it.
func TestValidate_DetectsBrokenPrevHashLink(t *testing.T) {
	bc := New(1)
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "alice", Amount: 100})
	bc.MinePending()
	bc.AddTransaction(ledger.Transaction{Sender: "alice", Recipient: "bob", Amount: 10})
	bc.MinePending()

	// Tamper with block 1's transaction, then re-mine it (attacker-effort)
	// so it individually satisfies the PoW target again.
	target := strings.Repeat("0", bc.Difficulty)
	tampered := bc.Blocks[1]
	tampered.Transactions[0].Amount = 90
	tampered.Nonce = 0
	tampered.Hash = tampered.CalculateHash()
	for !strings.HasPrefix(tampered.Hash, target) {
		tampered.Nonce++
		tampered.Hash = tampered.CalculateHash()
	}

	result := bc.Validate()
	if result.Valid {
		t.Fatal("expected chain to be invalid after tampering, even with a re-mined block")
	}
	if result.FailedHeight != 2 {
		t.Fatalf("expected the break to surface at block 2 (whose prev-hash is now stale), got %d",
			result.FailedHeight)
	}
}

// TestAddTransaction_OverspendRejectedAtMempool covers FR-4 at the mempool
// layer: AddTransaction itself must refuse an overspending transaction
// before it ever reaches a block.
func TestAddTransaction_OverspendRejectedAtMempool(t *testing.T) {
	bc := New(1)
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "alice", Amount: 100})
	bc.MinePending()

	err := bc.AddTransaction(ledger.Transaction{Sender: "alice", Recipient: "bob", Amount: 150})
	if err == nil {
		t.Fatal("expected overspending transaction to be rejected by AddTransaction")
	}
	if got := bc.Balances().Get("alice"); got != 100 {
		t.Fatalf("expected alice's balance unchanged at 100, got %d", got)
	}
}

// TestMinePending_RespectsMaxBlockSize covers FR-9: mining should only pull
// up to MaxBlockSize transactions from the pool, leaving the rest pending.
func TestMinePending_RespectsMaxBlockSize(t *testing.T) {
	bc := New(1)
	bc.MaxBlockSize = 2
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "a", Amount: 10})
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "b", Amount: 10})
	bc.AddTransaction(ledger.Transaction{Sender: "", Recipient: "c", Amount: 10})

	result, err := bc.MinePending()
	if err != nil {
		t.Fatalf("unexpected error mining: %v", err)
	}
	if len(result.Block.Transactions) != 2 {
		t.Fatalf("expected block to carry 2 transactions (MaxBlockSize), got %d", len(result.Block.Transactions))
	}
	if len(bc.Pending) != 1 {
		t.Fatalf("expected 1 transaction left pending, got %d", len(bc.Pending))
	}
}
