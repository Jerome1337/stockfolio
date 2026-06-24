package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"stockfolio/src/brokers"
	"stockfolio/src/finance"
	"stockfolio/src/helpers"
	"stockfolio/src/storage"
)

var logger *zap.Logger

func main() {
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()

	if err := godotenv.Load(".env"); err != nil {
		logger.Fatal("load .env", zap.Error(err))
	}

	importsDir := envOrDefault("IMPORTS_DIR", "./imports")

	logger.Info("stockfolioparsing CSVs...")
	txs, sells, divs, err := parseAllCSVs(importsDir)
	if err != nil {
		logger.Fatal("parse csvs", zap.Error(err))
	}
	logger.Info("found", zap.Int("transactions", len(txs)), zap.Int("sells", len(sells)), zap.Int("dividends", len(divs)))

	logger.Info("stockfoliofetching FX rates...")
	txs, err = applyFXRates(txs)
	if err != nil {
		logger.Fatal("fx rates", zap.Error(err))
	}

	logger.Info("stockfolioconnecting to GSheet...")
	gs, err := storage.New()
	if err != nil {
		logger.Fatal("storage", zap.Error(err))
	}

	logger.Info("stockfolioloading existing transactions (dedup check)...")
	existing, err := loadExistingTransactions(gs)
	if err != nil {
		logger.Fatal("load existing", zap.Error(err))
	}

	logger.Info("stockfolioupserting transactions...")
	newTxs := dedup(txs, existing)
	if len(newTxs) == 0 {
		logger.Info("no new transactions to insert")
	} else {
		if err := insertTransactions(gs, newTxs); err != nil {
			logger.Fatal("insert transactions", zap.Error(err))
		}
		logger.Info("inserted transactions", zap.Int("count", len(newTxs)))
	}

	logger.Info("stockfolioupserting dividends...")
	existingDivs, err := loadExistingDividends(gs)
	if err != nil {
		logger.Fatal("load existing dividends", zap.Error(err))
	}
	newDivs := dedupDividends(divs, existingDivs)
	if len(newDivs) == 0 {
		logger.Info("no new dividends to insert")
	} else {
		if err := insertDividends(gs, newDivs); err != nil {
			logger.Fatal("insert dividends", zap.Error(err))
		}
		logger.Info("inserted dividends", zap.Int("count", len(newDivs)))
	}

	logger.Info("stockfoliorecomputing positions tab...")
	if err := recomputePositions(gs, sells); err != nil {
		logger.Fatal("recompute positions", zap.Error(err))
	}

	logger.Info("✓ import complete")
}

// ── CSV parsing ───────────────────────────────────────────────────────────────

func parseAllCSVs(dir string) ([]brokers.Transaction, []brokers.SellRecord, []brokers.Dividend, error) {
	var txs []brokers.Transaction
	var sells []brokers.SellRecord
	var divs []brokers.Dividend

	degiroPath := dir + "/degiro.csv"
	if fileExists(degiroPath) {
		t, err := brokers.ParseDegiroExport(degiroPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("degiro: %w", err)
		}
		txs = append(txs, t...)

		s, err := brokers.ParseDegiroSells(degiroPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("degiro sells: %w", err)
		}
		sells = append(sells, s...)
		logger.Info("degiro", zap.Int("buys", len(t)), zap.Int("sells", len(s)))
	} else {
		logger.Info("degiro.csv not found, skipping")
	}

	trPath := dir + "/tr.csv"
	if fileExists(trPath) {
		t, err := brokers.ParseTRExport(trPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("tr: %w", err)
		}
		txs = append(txs, t...)

		s, err := brokers.ParseTRSells(trPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("tr sells: %w", err)
		}
		sells = append(sells, s...)

		d, err := brokers.ParseTRDividends(trPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("tr dividends: %w", err)
		}
		divs = append(divs, d...)
		logger.Info("tr", zap.Int("buys", len(t)), zap.Int("sells", len(s)), zap.Int("dividends", len(d)))
	} else {
		logger.Info("tr.csv not found, skipping")
	}

	return txs, sells, divs, nil
}

// ── FX conversion ─────────────────────────────────────────────────────────────

// applyFXRates converts all raw prices to EUR.
// Degiro already provides EUR amounts — PriceEUR is pre-filled, skip those.
// TR prices may be in USD — convert using today's rate (good enough for weekly reporting).
// Cache rates to avoid hammering the API.
func applyFXRates(txs []brokers.Transaction) ([]brokers.Transaction, error) {
	rateCache := map[string]float64{}

	for i, tx := range txs {
		// Degiro pre-fills PriceEUR — skip FX conversion
		if tx.PriceEUR > 0 {
			continue
		}

		if tx.RawCurrency == helpers.EUR {
			txs[i].PriceEUR = tx.RawPrice
			continue
		}

		cacheKey := tx.RawCurrency + "_EUR"
		rate, ok := rateCache[cacheKey]
		if !ok {
			var err error
			rate, err = finance.GetFXRate(tx.RawCurrency, helpers.EUR)
			if err != nil {
				return nil, fmt.Errorf("fx rate %s/EUR: %w", tx.RawCurrency, err)
			}
			rateCache[cacheKey] = rate
		}

		txs[i].PriceEUR = round2(tx.RawPrice * rate)
	}

	return txs, nil
}

// ── GSheet helpers ────────────────────────────────────────────────────────────

type txKey struct {
	date   string
	ticker string
	broker string
}

func loadExistingTransactions(gs *storage.Client) (map[txKey]bool, error) {
	rows, err := gs.ReadTab("transactions")
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return map[txKey]bool{}, nil
	}

	idx := storage.HeaderIndex(rows[0])
	existing := map[txKey]bool{}

	for _, row := range rows[1:] {
		if len(row) <= maxIdx(idx) {
			continue
		}
		k := txKey{
			date:   safeGet(row, idx, "date"),
			ticker: safeGet(row, idx, "ticker"),
			broker: safeGet(row, idx, "broker"),
		}
		existing[k] = true
	}
	return existing, nil
}

func dedup(txs []brokers.Transaction, existing map[txKey]bool) []brokers.Transaction {
	var out []brokers.Transaction
	for _, tx := range txs {
		k := txKey{
			date:   tx.Date.Format("2006-01-02"),
			ticker: tx.Ticker,
			broker: tx.Broker,
		}
		if !existing[k] {
			out = append(out, tx)
		}
	}
	return out
}

func insertTransactions(gs *storage.Client, txs []brokers.Transaction) error {
	rows := make([][]any, len(txs))
	for i, tx := range txs {
		rows[i] = []any{
			tx.Date.Format("2006-01-02"),
			tx.Ticker,
			tx.Type,
			tx.Broker,
			tx.Shares,
			tx.PriceEUR,
			tx.RawPrice,
			tx.RawCurrency,
		}
	}
	return gs.AppendRows("transactions", rows)
}

type divKey struct {
	date   string
	ticker string
}

func loadExistingDividends(gs *storage.Client) (map[divKey]bool, error) {
	rows, err := gs.ReadTab("dividends")
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return map[divKey]bool{}, nil
	}

	idx := storage.HeaderIndex(rows[0])
	existing := map[divKey]bool{}
	for _, row := range rows[1:] {
		k := divKey{
			date:   safeGet(row, idx, "date"),
			ticker: safeGet(row, idx, "ticker"),
		}
		existing[k] = true
	}
	return existing, nil
}

func dedupDividends(divs []brokers.Dividend, existing map[divKey]bool) []brokers.Dividend {
	var out []brokers.Dividend
	for _, d := range divs {
		k := divKey{date: d.Date.Format("2006-01-02"), ticker: d.Ticker}
		if !existing[k] {
			out = append(out, d)
		}
	}
	return out
}

func insertDividends(gs *storage.Client, divs []brokers.Dividend) error {
	rows := make([][]any, len(divs))
	for i, d := range divs {
		amountEUR := d.RawAmount
		if d.RawCurrency != helpers.EUR {
			rate, err := finance.GetFXRate(d.RawCurrency, helpers.EUR)
			if err == nil {
				amountEUR = round2(d.RawAmount * rate)
			}
		}
		rows[i] = []any{
			d.Date.Format("2006-01-02"),
			d.Ticker,
			amountEUR,
			d.RawAmount,
			d.RawCurrency,
			d.Broker,
		}
	}
	return gs.AppendRows("dividends", rows)
}

// ── Positions recompute ───────────────────────────────────────────────────────

// recomputePositions reads all transactions, computes weighted avg price
// per ticker, then rewrites the positions tab.
func recomputePositions(gs *storage.Client, sells []brokers.SellRecord) error {
	rows, err := gs.ReadTab("transactions")
	if err != nil {
		return err
	}
	if len(rows) < 2 {
		return nil
	}

	idx := storage.HeaderIndex(rows[0])

	type posAccum struct {
		totalShares   float64
		totalCostEUR  float64
		firstBuyDate  time.Time
		txType        string
		broker        string
		rawCurrency   string
	}

	accum := map[string]*posAccum{}

	for _, row := range rows[1:] {
		ticker := safeGet(row, idx, "ticker")
		if ticker == "" {
			continue
		}

		shares, _ := strconv.ParseFloat(safeGet(row, idx, "shares"), 64)
		priceEUR, _ := strconv.ParseFloat(safeGet(row, idx, "price_eur"), 64)
		dateStr := safeGet(row, idx, "date")
		date, _ := time.Parse("2006-01-02", dateStr)

		if _, ok := accum[ticker]; !ok {
			accum[ticker] = &posAccum{
				firstBuyDate: date,
				txType:       safeGet(row, idx, "type"),
				broker:       safeGet(row, idx, "broker"),
				rawCurrency:  safeGet(row, idx, "raw_currency"),
			}
		}

		p := accum[ticker]
		p.totalShares += shares
		p.totalCostEUR += shares * priceEUR
		if date.Before(p.firstBuyDate) {
			p.firstBuyDate = date
		}
	}

	// Aggregate sells per ticker to detect closed positions.
	type sellAccum struct {
		totalShares  float64
		lastSellDate time.Time
	}
	sellByTicker := map[string]*sellAccum{}
	for _, s := range sells {
		sa, ok := sellByTicker[s.Ticker]
		if !ok {
			sa = &sellAccum{lastSellDate: s.Date}
			sellByTicker[s.Ticker] = sa
		}
		sa.totalShares += s.Shares
		if s.Date.After(sa.lastSellDate) {
			sa.lastSellDate = s.Date
		}
	}

	// Read existing positions to preserve manually-set fields (type, benchmark_ticker)
	benchmarks := map[string]string{}
	existingTypes := map[string]string{}
	existingPos, err := gs.ReadTab("positions")
	if err == nil && len(existingPos) > 1 {
		posIdx := storage.HeaderIndex(existingPos[0])
		for _, row := range existingPos[1:] {
			ticker := safeGet(row, posIdx, "ticker")
			if ticker == "" {
				continue
			}
			benchmarks[ticker] = safeGet(row, posIdx, "benchmark_ticker")
			if t := safeGet(row, posIdx, "type"); t != "" {
				existingTypes[ticker] = t
			}
		}
	}

	// Sort tickers for stable output
	tickers := make([]string, 0, len(accum))
	for t := range accum {
		tickers = append(tickers, t)
	}
	sort.Strings(tickers)

	// Build new positions rows
	header := []any{
		"ticker", "type", "broker", "shares",
		"avg_buy_price_eur", "first_buy_date", "currency", "benchmark_ticker", "closing_date",
	}
	newRows := [][]any{header}

	for _, ticker := range tickers {
		p := accum[ticker]
		avgPrice := 0.0
		if p.totalShares > 0 {
			avgPrice = round2(p.totalCostEUR / p.totalShares)
		}
		posType := p.txType
		if existing, ok := existingTypes[ticker]; ok {
			posType = existing // ponytail: never overwrite manually-fixed types
		}
		closingDate := ""
		if sa, ok := sellByTicker[ticker]; ok {
			netShares := p.totalShares - sa.totalShares
			if netShares < 0.001 {
				closingDate = sa.lastSellDate.Format("2006-01-02")
			}
		}
		netShares := p.totalShares
		if sa, ok := sellByTicker[ticker]; ok {
			netShares -= sa.totalShares
		}
		newRows = append(newRows, []any{
			ticker,
			posType,
			p.broker,
			round2(netShares),
			avgPrice,
			p.firstBuyDate.Format("2006-01-02"),
			p.rawCurrency,
			benchmarks[ticker],
			closingDate,
		})
	}

	return gs.ClearAndWrite("positions", newRows)
}

// ── utils ─────────────────────────────────────────────────────────────────────

func safeGet(row []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func maxIdx(idx map[string]int) int {
	max := 0
	for _, v := range idx {
		if v > max {
			max = v
		}
	}
	return max
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
