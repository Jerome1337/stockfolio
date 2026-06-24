package report

import (
	"fmt"
	"time"

	"stockfolio/src/brokers"
	"stockfolio/src/finance"
	"stockfolio/src/helpers"
	"stockfolio/src/storage"
)

type Report struct {
	Date              time.Time
	Positions         []storage.Position
	TotalInvestedEUR  float64
	TotalValueEUR     float64
	TotalPnLPct       float64
	DividendsMTD      float64
	DividendsYTD      float64
	Flags             []storage.Position
	ETFs              []storage.Position
	Stocks            []storage.Position
	MarketWeekPct     float64 // S&P 500 week change
}

// Build loads positions from GSheet, enriches them with live data, returns Report.
func Build(gs *storage.Client) (*Report, error) {
	isinMap, err := helpers.LoadISINMap("isin_map.json")

	if err != nil {
		return nil, fmt.Errorf("load isin map: %w", err)
	}

	positions, err := storage.LoadPositions(gs)
	if err != nil {
		return nil, fmt.Errorf("load positions: %w", err)
	}

	divYTD, divMTD, divByTicker, err := storage.LoadDividends(gs)
	if err != nil {
		return nil, fmt.Errorf("load dividends: %w", err)
	}

	spQuote, err := finance.GetQuote(brokers.SP500_TICKER)
	if err != nil {
		spQuote = &finance.Quote{WeekChangePct: 0}
	}

	fxCache := map[string]float64{}

	for i := range positions {
		p := &positions[i]

		yahooTicker := p.Ticker
		if mapped, ok := isinMap[p.Ticker]; ok {
			yahooTicker = mapped
		}

		quote, err := finance.GetQuote(yahooTicker)
		if err != nil {
			p.Flag = "REVIEW"
			p.FlagReason = fmt.Sprintf("could not fetch price: %v", err)

			continue
		}

		priceEUR := quote.CurrentPrice

		if quote.Currency != helpers.EUR {
			rate, ok := fxCache[quote.Currency]

			if !ok {
				rate, err = finance.GetFXRate(quote.Currency, helpers.EUR)

				if err != nil {
					rate = 1.0
				}

				fxCache[quote.Currency] = rate
			}
			priceEUR = quote.CurrentPrice * rate
		}

		p.Name = quote.ShortName
		p.CurrentPriceEUR = helpers.Round2(priceEUR)
		p.ValueEUR = helpers.Round2(p.Shares * priceEUR)
		p.InvestedEUR = helpers.Round2(p.Shares * p.AvgBuyPriceEUR)
		p.WeekChangePct = helpers.Round2(quote.WeekChangePct)

		if p.InvestedEUR > 0 {
			p.PnLPct = helpers.Round2(((p.ValueEUR - p.InvestedEUR) / p.InvestedEUR) * 100)
		}

		if p.Type == brokers.ETF && p.BenchmarkTicker != "" {
			benchPnL, err := benchmarkPnLSince(p.BenchmarkTicker, p.FirstBuyDate, fxCache)
			if err == nil {
				p.BenchmarkPnLPct = helpers.Round2(benchPnL)
			}
		}

		if p.Type == brokers.STOCK {
			divs := divByTicker[p.Ticker]

			p.DividendsYTD = helpers.Round2(divs.Ytd)
			p.DividendsMTD = helpers.Round2(divs.Mtd)

			annualDiv, err := finance.GetAnnualDividend(yahooTicker)

			if err == nil && p.AvgBuyPriceEUR > 0 {
				if quote.Currency != helpers.EUR {
					annualDiv *= fxCache[quote.Currency]
				}
				p.YieldOnCost = helpers.Round2((annualDiv / p.AvgBuyPriceEUR) * 100)
			}
		}

		p.Flag, p.FlagReason = computeFlag(p)
	}

	return buildReport(positions, divYTD, divMTD, spQuote.WeekChangePct), nil
}

func buildReport(positions []storage.Position, divYTD, divMTD, marketWeekPct float64) *Report {
	r := &Report{
		Date:          time.Now(),
		Positions:     positions,
		DividendsYTD:  divYTD,
		DividendsMTD:  divMTD,
		MarketWeekPct: helpers.Round2(marketWeekPct),
	}

	for _, p := range positions {
		r.TotalInvestedEUR += p.InvestedEUR
		r.TotalValueEUR += p.ValueEUR

		if p.Type == brokers.ETF {
			r.ETFs = append(r.ETFs, p)
		} else {
			r.Stocks = append(r.Stocks, p)
		}

		if p.Flag != "" {
			r.Flags = append(r.Flags, p)
		}
	}

	r.TotalInvestedEUR = helpers.Round2(r.TotalInvestedEUR)
	r.TotalValueEUR = helpers.Round2(r.TotalValueEUR)

	if r.TotalInvestedEUR > 0 {
		r.TotalPnLPct = helpers.Round2(((r.TotalValueEUR - r.TotalInvestedEUR) / r.TotalInvestedEUR) * 100)
	}

	return r
}

func WriteReportOutput(gs *storage.Client, r *Report, text string) error {
	flags := ""

	for i, f := range r.Flags {
		if i > 0 {
			flags += ","
		}
		flags += f.Ticker
	}

	rows := [][]any{
		{"key", "value"},
		{"report_date", r.Date.Format("2006-01-02")},
		{"total_invested_eur", r.TotalInvestedEUR},
		{"total_value_eur", r.TotalValueEUR},
		{"total_pnl_pct", r.TotalPnLPct},
		{"dividends_month_eur", r.DividendsMTD},
		{"dividends_ytd_eur", r.DividendsYTD},
		{"market_week_pct", r.MarketWeekPct},
		{"flags", flags},
		{"generated_at", time.Now().Format(time.RFC3339)},
		{"report_text", text},
	}

	return gs.ClearAndWrite("report_output", rows)
}
