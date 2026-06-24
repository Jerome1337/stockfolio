package brokers

import (
	"fmt"
	"strings"

	"stockfolio/src/csv"
	"stockfolio/src/helpers"
)

const TRADE_REPUBLIC = "traderepublic"

// ParseTRExport parses a Trade Republic CSV export.
// Expected headers:
// datetime | date | account_type | category | type | asset_class | name |
// symbol | shares | price | amount | fee | tax | currency |
// original_amount | original_currency | fx_rate | description |
// transaction_id | counterparty_name | counterparty_iban | payment_reference | mcc_c
func ParseTRExport(path string) ([]Transaction, error) {
	rows, headers, err := csv.ReadCSV(path)
	if err != nil {
		return nil, fmt.Errorf("tr: %w", err)
	}

	idx := csv.HeaderIndex(headers)

	required := []string{"datetime", "symbol", "shares", "price", "currency", "asset_class", "type"}
	if err := csv.CheckHeaders(idx, required); err != nil {
		return nil, fmt.Errorf("tr headers: %w", err)
	}

	var txs []Transaction
	for i, row := range rows {
		if len(row) < len(headers) {
			continue
		}

		txType := strings.ToLower(helpers.Clean(row[idx["type"]]))
		if txType != "buy" && txType != "savings_plan" {
			continue
		}

		symbol := helpers.Clean(row[idx["symbol"]])
		if symbol == "" {
			continue
		}

		shares, err := helpers.ParseFloat(row[idx["shares"]])
		if err != nil {
			return nil, fmt.Errorf("tr row %d: shares: %w", i+2, err)
		}

		price, err := helpers.ParseFloat(row[idx["price"]])
		if err != nil {
			return nil, fmt.Errorf("tr row %d: price: %w", i+2, err)
		}

		currency := helpers.Clean(row[idx["currency"]])

		date, err := helpers.FlexParseDate(helpers.Clean(row[idx["datetime"]]))
		if err != nil {
			return nil, fmt.Errorf("tr row %d: datetime: %w", i+2, err)
		}

		assetClass := strings.ToLower(helpers.Clean(row[idx["asset_class"]]))
		posType := classifyAsset(assetClass)

		txs = append(txs, Transaction{
			Date:        date,
			Ticker:      symbol,
			Type:        posType,
			Broker:      TRADE_REPUBLIC,
			Shares:      shares,
			RawPrice:    price,
			RawCurrency: currency,
		})
	}

	return txs, nil
}

func ParseTRSells(path string) ([]SellRecord, error) {
	rows, headers, err := csv.ReadCSV(path)
	if err != nil {
		return nil, fmt.Errorf("tr sells: %w", err)
	}

	idx := csv.HeaderIndex(headers)

	var sells []SellRecord
	for i, row := range rows {
		if len(row) < len(headers) {
			continue
		}

		if strings.ToLower(helpers.Clean(row[idx["type"]])) != "sell" {
			continue
		}

		symbol := helpers.Clean(row[idx["symbol"]])
		if symbol == "" {
			continue
		}

		shares, err := helpers.ParseFloat(row[idx["shares"]])
		if err != nil {
			return nil, fmt.Errorf("tr sell row %d: shares: %w", i+2, err)
		}

		if shares < 0 {
			shares = -shares
		}

		date, err := helpers.FlexParseDate(helpers.Clean(row[idx["datetime"]]))
		if err != nil {
			return nil, fmt.Errorf("tr sell row %d: datetime: %w", i+2, err)
		}

		sells = append(sells, SellRecord{Date: date, Ticker: symbol, Shares: shares})
	}

	return sells, nil
}

func ParseTRDividends(path string) ([]Dividend, error) {
	rows, headers, err := csv.ReadCSV(path)
	if err != nil {
		return nil, fmt.Errorf("tr dividends: %w", err)
	}

	idx := csv.HeaderIndex(headers)

	var divs []Dividend
	for _, row := range rows {
		if len(row) < len(headers) {
			continue
		}

		txType := strings.ToLower(helpers.Clean(row[idx["type"]]))
		if txType != "dividend" {
			continue
		}

		symbol := helpers.Clean(row[idx["symbol"]])
		if symbol == "" {
			continue
		}

		amount, err := helpers.ParseFloat(row[idx["amount"]])
		if err != nil {
			continue
		}

		currency := helpers.Clean(row[idx["currency"]])

		date, err := helpers.FlexParseDate(helpers.Clean(row[idx["datetime"]]))
		if err != nil {
			continue
		}

		divs = append(divs, Dividend{
			Date:        date,
			Ticker:      symbol,
			RawAmount:   amount,
			RawCurrency: currency,
			Broker:      TRADE_REPUBLIC,
		})
	}

	return divs, nil
}
