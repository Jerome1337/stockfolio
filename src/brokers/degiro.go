package brokers

import (
	"fmt"

	"stockfolio/src/csv"
	"stockfolio/src/helpers"
)

const DEGIRO = "degiro"

// ParseDegiroExport parses a Degiro transaction history CSV export (FR locale).
// Headers:
// Date | Heure | Produit | Code ISIN | Place boursière sectionnée |
// Lieu d'exécution | Quantité | Cours | (currency col) | Montant devise locale |
// (currency col) | Montant EUR | Taux de change | Frais conversion AutoFX |
// Frais de courtage et/ou de parties | Montant négocié EUR | ID O
//
// Note: Degiro inserts unnamed currency columns after Cours and after
// Montant devise locale — we use column index for those, name for the rest.
func ParseDegiroExport(path string) ([]Transaction, error) {
	rows, headers, err := csv.ReadCSV(path)

	if err != nil {
		return nil, fmt.Errorf("degiro: %w", err)
	}

	idx := csv.HeaderIndex(headers)
	required := []string{"Date", "Code ISIN", "Quantité", "Montant EUR"}

	if err := csv.CheckHeaders(idx, required); err != nil {
		return nil, fmt.Errorf("degiro headers: %w", err)
	}

	coursIdx := idx["Cours"]

	var txs []Transaction
	for i, row := range rows {
		if len(row) <= coursIdx {
			continue
		}

		isin := helpers.Clean(row[idx["Code ISIN"]])
		if isin == "" {
			continue
		}

		sharesRaw, err := helpers.ParseFloat(row[idx["Quantité"]])

		if err != nil {
			return nil, fmt.Errorf("degiro row %d: quantité: %w", i+2, err)
		}

		if sharesRaw <= 0 {
			continue
		}

		amountEUR, err := helpers.ParseFloat(row[idx["Montant EUR"]])
		if err != nil {
			return nil, fmt.Errorf("degiro row %d: montant EUR: %w", i+2, err)
		}

		// Degiro shows buy amounts as negative (cash outflow) — make positive
		if amountEUR < 0 {
			amountEUR = -amountEUR
		}

		rawPrice := 0.0
		if coursIdx < len(row) {
			rawPrice, _ = helpers.ParseFloat(row[coursIdx])
		}

		priceEUR := 0.0
		if sharesRaw > 0 {
			priceEUR = amountEUR / sharesRaw
		}

		date, err := helpers.FlexParseDate(helpers.Clean(row[idx["Date"]]))
		if err != nil {
			return nil, fmt.Errorf("degiro row %d: date: %w", i+2, err)
		}

		ticker := isin

		txs = append(txs, Transaction{
			Date:        date,
			Ticker:      ticker,
			Type:        "ETF",
			Broker:      DEGIRO,
			Shares:      sharesRaw,
			RawPrice:    rawPrice,
			RawCurrency: helpers.EUR,
			PriceEUR:    priceEUR,
		})
	}

	return txs, nil
}

// ParseDegiroSells extracts sell rows from a Degiro CSV export (negative Quantity).
func ParseDegiroSells(path string) ([]SellRecord, error) {
	rows, headers, err := csv.ReadCSV(path)
	if err != nil {
		return nil, fmt.Errorf("degiro sells: %w", err)
	}

	idx := csv.HeaderIndex(headers)
	if err := csv.CheckHeaders(idx, []string{"Date", "Code ISIN", "Quantité"}); err != nil {
		return nil, fmt.Errorf("degiro sell headers: %w", err)
	}

	var sells []SellRecord
	for i, row := range rows {
		isin := helpers.Clean(row[idx["Code ISIN"]])
		if isin == "" {
			continue
		}

		shares, err := helpers.ParseFloat(row[idx["Quantité"]])
		if err != nil {
			return nil, fmt.Errorf("degiro sell row %d: quantité: %w", i+2, err)
		}

		if shares >= 0 {
			continue
		}

		date, err := helpers.FlexParseDate(helpers.Clean(row[idx["Date"]]))
		if err != nil {
			return nil, fmt.Errorf("degiro sell row %d: date: %w", i+2, err)
		}

		sells = append(sells, SellRecord{Date: date, Ticker: isin, Shares: -shares})
	}
	return sells, nil
}
