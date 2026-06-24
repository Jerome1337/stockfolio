package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Snapshot struct {
	Date          string             `json:"date"`
	TotalValueEUR float64            `json:"total_value_eur"`
	TotalPnLPct   float64            `json:"total_pnl_pct"`
	Positions     map[string]float64 `json:"positions"`
}

func LoadLatestSnapshot(dir, currentFile string) (*Snapshot, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var files []string

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == currentFile || !strings.HasPrefix(name, "week_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		files = append(files, name)
	}

	if len(files) == 0 {
		return nil, nil
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	b, err := os.ReadFile(filepath.Join(dir, files[0]))
	if err != nil {
		return nil, err
	}

	var s Snapshot
	return &s, json.Unmarshal(b, &s)
}

func SaveSnapshot(dir string, r *Report) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	s := Snapshot{
		Date:          r.Date.Format("2006-01-02"),
		TotalValueEUR: r.TotalValueEUR,
		TotalPnLPct:   r.TotalPnLPct,
		Positions:     make(map[string]float64, len(r.Positions)),
	}

	for _, p := range r.Positions {
		s.Positions[p.Ticker] = p.PnLPct
	}

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "week_"+r.Date.Format("2006-01-02")+".json"), b, 0644)
}
