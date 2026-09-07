package finance

import (
	"encoding/json"
	"fmt"
	"time"

	"stockfolio/src/http"
)

const yahooQueryURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

type Quote struct {
	Ticker        string
	ShortName     string
	CurrentPrice  float64
	Currency      string
	PreviousClose float64
	WeekChangePct float64
}

type FXRate struct {
	From string
	To   string
	Rate float64
}

type YahooQuote struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency             string  `json:"currency"`
				ShortName            string  `json:"shortName"`
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				ChartPreviousClose   float64 `json:"chartPreviousClose"`
			} `json:"meta"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"chart"`
}

type YahooChart struct {
	Chart struct {
		Result []struct {
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
			Meta struct {
				TrailingAnnualDividendRate float64 `json:"trailingAnnualDividendRate"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

func GetQuote(ticker string) (*Quote, error) {
	url := fmt.Sprintf("%s%s?interval=1wk&range=1mo", yahooQueryURL, ticker)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch quote %s: %w", ticker, err)
	}

	defer resp.Body.Close()

	var yahooQuote YahooQuote

	if err := json.NewDecoder(resp.Body).Decode(&yahooQuote); err != nil {
		return nil, fmt.Errorf("decode quote %s: %w", ticker, err)
	}

	if len(yahooQuote.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data for ticker %s", ticker)
	}

	r := yahooQuote.Chart.Result[0]
	current := r.Meta.RegularMarketPrice
	prev := r.Meta.ChartPreviousClose

	weekChange := 0.0
	if prev > 0 {
		weekChange = ((current - prev) / prev) * 100
	}

	return &Quote{
		Ticker:        ticker,
		ShortName:     r.Meta.ShortName,
		CurrentPrice:  current,
		Currency:      r.Meta.Currency,
		PreviousClose: prev,
		WeekChangePct: weekChange,
	}, nil
}

func GetHistoricalPrice(ticker string, date time.Time) (float64, error) {
	start := date.Unix()
	end := date.Add(48 * time.Hour).Unix() // +2 days buffer for weekends

	url := fmt.Sprintf("%s%s?interval=1d&period1=%d&period2=%d", yahooQueryURL, ticker, start, end)
	resp, err := http.Get(url)

	if err != nil {
		return 0, fmt.Errorf("fetch historical %s: %w", ticker, err)
	}

	defer resp.Body.Close()

	var yahooChart YahooChart

	if err := json.NewDecoder(resp.Body).Decode(&yahooChart); err != nil {
		return 0, fmt.Errorf("decode historical %s: %w", ticker, err)
	}

	if len(yahooChart.Chart.Result) == 0 {
		return 0, fmt.Errorf("no historical data for %s on %s", ticker, date.Format("2006-01-02"))
	}

	closes := yahooChart.Chart.Result[0].Indicators.Quote
	if len(closes) == 0 || len(closes[0].Close) == 0 {
		return 0, fmt.Errorf("empty close prices for %s", ticker)
	}

	return closes[0].Close[0], nil
}

func GetAnnualDividend(ticker string) (float64, error) {
	url := fmt.Sprintf("%s%s?interval=1d&range=1y", yahooQueryURL, ticker)
	resp, err := http.Get(url)

	if err != nil {
		return 0, fmt.Errorf("fetch dividend %s: %w", ticker, err)
	}

	defer resp.Body.Close()

	var yahooChart YahooChart

	if err := json.NewDecoder(resp.Body).Decode(&yahooChart); err != nil {
		return 0, fmt.Errorf("decode dividend %s: %w", ticker, err)
	}

	if len(yahooChart.Chart.Result) == 0 {
		return 0, fmt.Errorf("no data for %s", ticker)
	}

	return yahooChart.Chart.Result[0].Meta.TrailingAnnualDividendRate, nil
}

const yahooSearchURL = "https://query1.finance.yahoo.com/v1/finance/search?quotesCount=1&newsCount=0&q="

func ResolveSymbol(isin string) (string, error) {
	resp, err := http.Get(yahooSearchURL + isin)
	if err != nil {
		return "", fmt.Errorf("search %s: %w", isin, err)
	}

	defer resp.Body.Close()

	var search struct {
		Quotes []struct {
			Symbol string `json:"symbol"`
		} `json:"quotes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&search); err != nil {
		return "", fmt.Errorf("decode search %s: %w", isin, err)
	}

	if len(search.Quotes) == 0 || search.Quotes[0].Symbol == "" {
		return "", fmt.Errorf("no symbol found for %s", isin)
	}

	return search.Quotes[0].Symbol, nil
}
