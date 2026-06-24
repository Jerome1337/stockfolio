package report

import (
	"fmt"
	"math"
	"sort"
	"time"

	"stockfolio/src/brokers"
	"stockfolio/src/finance"
	"stockfolio/src/storage"
)

// DisplayName returns "Name (TICKER)" if name available, else just "TICKER".
func DisplayName(p storage.Position) string {
	if p.Name != "" && p.Name != p.Ticker {
		name := p.Name

		if len(name) > 16 {
			name = name[:15] + "..."
		}

		return fmt.Sprintf("%s (%s)", name, p.Ticker)
	}

	return p.Ticker
}

func benchmarkPnLSince(ticker string, since time.Time, fxCache map[string]float64) (float64, error) {
	historical, err := finance.GetHistoricalPrice(ticker, since)

	if err != nil {
		return 0, err
	}

	if historical == 0 {
		return 0, fmt.Errorf("historical price is zero")
	}

	current, err := finance.GetQuote(ticker)

	if err != nil {
		return 0, err
	}

	return ((current.CurrentPrice - historical) / historical) * 100, nil
}

func SortedByPnL(positions []storage.Position) []storage.Position {
	out := make([]storage.Position, len(positions))

	copy(out, positions)

	sort.Slice(out, func(i, j int) bool {
		return out[i].PnLPct > out[j].PnLPct
	})

	return out
}

func computeFlag(p *storage.Position) (string, string) {
	const (
		REVIEW_FLAG = "REVIEW"
		WATCH_FLAG = "WATCH"
	)

	if p.Type == brokers.ETF && p.BenchmarkTicker != "" {
		diff := p.PnLPct - p.BenchmarkPnLPct

		if diff < -10 {
			return REVIEW_FLAG, fmt.Sprintf("behind benchmark by %.1f%% since first buy", math.Abs(diff))
		}

		if diff < -5 {
			return WATCH_FLAG, fmt.Sprintf("lagging benchmark by %.1f%%", math.Abs(diff))
		}
	}

	if p.Type == brokers.STOCK {
		if p.PnLPct <= -20 && p.YieldOnCost < 3 {
			return REVIEW_FLAG, fmt.Sprintf("down %.1f%% with only %.1f%% yield on cost", math.Abs(p.PnLPct), p.YieldOnCost)
		}

		if p.PnLPct <= -10 {
			return WATCH_FLAG, fmt.Sprintf("down %.1f%% — monitor", math.Abs(p.PnLPct))
		}
	}

	if p.PnLPct >= -3 && p.PnLPct <= 5 && p.PnLPct < 0 {
		return WATCH_FLAG, fmt.Sprintf("close to breakeven (%.1f%%) — exit opportunity?", p.PnLPct)
	}

	return "", ""
}
