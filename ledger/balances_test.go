package ledger

import "testing"

// TestApply_OverspendRejected covers FR-4 / the Gherkin scenario "An
// overspending transaction is rejected": an account with balance 100 cannot
// send 150, and the rejection must leave the balance untouched.
func TestApply_OverspendRejected(t *testing.T) {
	b := NewBalances()
	// Seed alice with 100 via a coinbase (no-sender) transaction.
	if err := b.Apply(Transaction{Sender: "", Recipient: "alice", Amount: 100}); err != nil {
		t.Fatalf("unexpected error seeding balance: %v", err)
	}

	err := b.Apply(Transaction{Sender: "alice", Recipient: "bob", Amount: 150})
	if err == nil {
		t.Fatal("expected overspend to be rejected, got nil error")
	}
	if got := b.Get("alice"); got != 100 {
		t.Fatalf("expected alice's balance unchanged at 100, got %d", got)
	}
	if got := b.Get("bob"); got != 0 {
		t.Fatalf("expected bob's balance unchanged at 0, got %d", got)
	}
}

// TestApply_NonPositiveAmountRejected covers the "malformed" half of FR-4.
func TestApply_NonPositiveAmountRejected(t *testing.T) {
	b := NewBalances()
	b.Apply(Transaction{Sender: "", Recipient: "alice", Amount: 100})

	cases := []int64{0, -5}
	for _, amt := range cases {
		err := b.Apply(Transaction{Sender: "alice", Recipient: "bob", Amount: amt})
		if err == nil {
			t.Fatalf("expected amount %d to be rejected", amt)
		}
	}
}

// TestApply_ValidTransferUpdatesBalances is the happy path: a legitimate
// transfer debits the sender and credits the recipient.
func TestApply_ValidTransferUpdatesBalances(t *testing.T) {
	b := NewBalances()
	b.Apply(Transaction{Sender: "", Recipient: "alice", Amount: 100})

	if err := b.Apply(Transaction{Sender: "alice", Recipient: "bob", Amount: 40}); err != nil {
		t.Fatalf("unexpected error on valid transfer: %v", err)
	}
	if got := b.Get("alice"); got != 60 {
		t.Fatalf("expected alice=60, got %d", got)
	}
	if got := b.Get("bob"); got != 40 {
		t.Fatalf("expected bob=40, got %d", got)
	}
}
