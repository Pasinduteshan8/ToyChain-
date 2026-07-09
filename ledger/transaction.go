// Package ledger holds the transaction type and (later) account-balance logic.
package ledger

// Transaction is the smallest unit of value transfer in the toy chain.
// Sender is empty for a coinbase/faucet transaction that mints new funds.
type Transaction struct {
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Amount    int64  `json:"amount"`
}
