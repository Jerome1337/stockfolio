package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

const DELIMITER = ','

func ReadCSV(path string) ([][]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1  // allow variable number of fields per row
	r.Comma = DELIMITER

	all, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("parse csv: %w", err)
	}

	if len(all) < 2 {
		return nil, nil, fmt.Errorf("csv has no data rows")
	}

	headers := all[0]
	rows := all[1:]

	return rows, headers, nil
}

func HeaderIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))

	for i, h := range headers {
		idx[strings.TrimSpace(h)] = i
	}

	return idx
}

func CheckHeaders(idx map[string]int, required []string) error {
	var missing []string

	for _, h := range required {
		if _, ok := idx[h]; !ok {
			missing = append(missing, h)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing columns: %s", strings.Join(missing, ", "))
	}

	return nil
}
