package finance

import (
	"encoding/json"
	"fmt"

	"stockfolio/src/http"
)

const frankfurterApiURL = "https://api.frankfurter.app/latest"

type FrankfurterRate struct {
	Rates map[string]float64 `json:"rates"`
}

func GetFXRate(from, to string) (float64, error) {
	if from == to {
		return 1.0, nil
	}

	url := fmt.Sprintf("%s?from=%s&to=%s", frankfurterApiURL, from, to)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("fetch fx %s/%s: %w", from, to, err)
	}

	defer resp.Body.Close()

	var frankfurterRate FrankfurterRate

	if err := json.NewDecoder(resp.Body).Decode(&frankfurterRate); err != nil {
		return 0, fmt.Errorf("decode fx: %w", err)
	}

	rate, ok := frankfurterRate.Rates[to]
	if !ok {
		return 0, fmt.Errorf("rate %s not found", to)
	}

	return rate, nil
}
