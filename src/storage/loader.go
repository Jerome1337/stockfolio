package storage

import (
	"fmt"
	"strconv"
	"time"

	"stockfolio/src/helpers"
)

type Position struct {
	Ticker          string
	Name            string
	Type            string
	Broker          string
	Shares          float64
	AvgBuyPriceEUR  float64
	FirstBuyDate    time.Time
	Currency        string
	BenchmarkTicker string

	CurrentPriceEUR float64
	ValueEUR        float64
	InvestedEUR     float64
	PnLPct          float64
	WeekChangePct   float64
	BenchmarkPnLPct float64 // since first buy, for ETFs only
	YieldOnCost     float64 // annual dividend / avg buy price, stocks only
	DividendsYTD    float64
	DividendsMTD    float64
	Flag            string // "", "WATCH", "REVIEW"
	FlagReason      string
}

type DividendAccumulator struct {
	Ytd float64
	Mtd float64
}


func LoadPositions(gs *Client) ([]Position, error) {
	rows, err := gs.ReadTab("positions")

	if err != nil {
		return nil, err
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("positions tab is empty")
	}

	idx := HeaderIndex(rows[0])

	var out []Position
	for _, row := range rows[1:] {
		ticker := SafeGetValue(row, idx, "ticker")

		if ticker == "" {
			continue
		}

		if SafeGetValue(row, idx, "closing_date") != "" {
			continue
		}

		shares, _ := strconv.ParseFloat(SafeGetValue(row, idx, "shares"), 64)
		avgPrice, _ := strconv.ParseFloat(SafeGetValue(row, idx, "avg_buy_price_eur"), 64)
		firstBuy, _ := time.Parse("2006-01-02", SafeGetValue(row, idx, "first_buy_date"))

		out = append(out, Position{
			Ticker:          ticker,
			Type:            SafeGetValue(row, idx, "type"),
			Broker:          SafeGetValue(row, idx, "broker"),
			Shares:          shares,
			AvgBuyPriceEUR:  avgPrice,
			FirstBuyDate:    firstBuy,
			Currency:        SafeGetValue(row, idx, "currency"),
			BenchmarkTicker: SafeGetValue(row, idx, "benchmark_ticker"),
		})
	}

	return out, nil
}

func LoadDividends(gs *Client) (float64, float64, map[string]DividendAccumulator, error) {
	rows, err := gs.ReadTab("dividends")

	if err != nil {
		return 0, 0, nil, err
	}

	now := time.Now()
	currentYear := now.Year()
	currentMonth := now.Month()

	byTicker := map[string]DividendAccumulator{}
	totalYTD := 0.0
	totalMTD := 0.0

	if len(rows) < 2 {
		return 0, 0, byTicker, nil
	}

	idx := HeaderIndex(rows[0])

	for _, row := range rows[1:] {
		ticker := SafeGetValue(row, idx, "ticker")
		amountStr := SafeGetValue(row, idx, "amount_eur")
		dateStr := SafeGetValue(row, idx, "date")

		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			continue
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if date.Year() != currentYear {
			continue
		}

		acc := byTicker[ticker]
		acc.Ytd += amount
		totalYTD += amount

		if date.Month() == currentMonth {
			acc.Mtd += amount
			totalMTD += amount
		}

		byTicker[ticker] = acc
	}

	return helpers.Round2(totalYTD), helpers.Round2(totalMTD), byTicker, nil
}
