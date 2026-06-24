package helpers

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadISINMap(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)

	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}

		return nil, fmt.Errorf("read isin_map.json: %w", err)
	}

	var m map[string]string

	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse isin_map.json: %w", err)
	}

	return m, nil
}
