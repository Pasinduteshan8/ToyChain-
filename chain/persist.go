package chain

import (
	"encoding/json"
	"os"
)

// Save writes the entire chain state (blocks, difficulty, pending pool,
// max block size) to path as JSON (FR-8).
func (bc *Blockchain) Save(path string) error {
	data, err := json.MarshalIndent(bc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads a previously saved chain from path. If the file does not
// exist, it returns a fresh chain at the given difficulty instead of an
// error, so first-run and normal-run are the same code path in the CLI.
func Load(path string, defaultDifficulty int) (*Blockchain, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(defaultDifficulty), nil
	}
	if err != nil {
		return nil, err
	}

	var bc Blockchain
	if err := json.Unmarshal(data, &bc); err != nil {
		return nil, err
	}
	return &bc, nil
}
