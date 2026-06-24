package brokers

import (
	"strings"
	"time"
)
const (
	ETF = "ETF"
	STOCK = "STOCK"
)

const SP500_TICKER = "^GSPC"

// Transaction is the normalized struct used by both parsers.
type Transaction struct {
	Date         time.Time
	Ticker       string
	Type         string // ETF or STOCK
	Broker       string
	Shares       float64
	RawPrice     float64
	RawCurrency  string
	PriceEUR     float64
}

// SellRecord is a minimal sell event used to detect closed positions.
type SellRecord struct {
	Date   time.Time
	Ticker string
	Shares float64
}

// Returns ticker, date, amount, currency to be logged in dividends tab.
type Dividend struct {
	Date        time.Time
	Ticker      string
	RawAmount   float64
	RawCurrency string
	Broker      string
}


func classifyAsset(assetClass string) string {
	switch {
	case strings.Contains(strings.ToLower(assetClass), "etf"), strings.Contains(strings.ToLower(assetClass), "fund"):
		return ETF
	default:
		return STOCK
	}
}
