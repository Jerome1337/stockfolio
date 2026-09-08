package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Client struct {
	srv     *sheets.Service
	sheetID string
}

func New() (*Client, error) {
	ctx := context.Background()

	sheetID := os.Getenv("SHEET_ID")

	b := []byte(os.Getenv("GOOGLE_CREDENTIALS_JSON"))

	if len(b) == 0 {
		credPath := os.Getenv("GOOGLE_CREDENTIALS_PATH")
		if credPath == "" {
			return nil, fmt.Errorf("set GOOGLE_CREDENTIALS_JSON or GOOGLE_CREDENTIALS_PATH")
		}

		var err error

		b, err = os.ReadFile(credPath)
		if err != nil {
			return nil, fmt.Errorf("read credentials: %w", err)
		}
	}

	cfg, err := google.JWTConfigFromJSON(b, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("parse service account: %w", err)
	}

	httpClient := cfg.Client(ctx)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create sheets service: %w", err)
	}

	return &Client{srv: srv, sheetID: sheetID}, nil
}

func (c *Client) ReadTab(tab string) ([][]string, error) {
	resp, err := c.srv.Spreadsheets.Values.Get(c.sheetID, tab).Do()
	if err != nil {
		return nil, fmt.Errorf("read tab %s: %w", tab, err)
	}

	rows := make([][]string, len(resp.Values))

	for i, row := range resp.Values {
		cells := make([]string, len(row))

		for j, cell := range row {
			cells[j] = fmt.Sprintf("%v", cell)
		}

		rows[i] = cells
	}

	return rows, nil
}

func HeaderIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))

	for i, h := range headers {
		idx[h] = i
	}

	return idx
}

func (c *Client) AppendRows(tab string, rows [][]any) error {
	vr := &sheets.ValueRange{Values: rows}

	_, err := c.srv.Spreadsheets.Values.
		Append(c.sheetID, tab, vr).
		ValueInputOption("USER_ENTERED").
		Do()

	if err != nil {
		return fmt.Errorf("append to %s: %w", tab, err)
	}

	return nil
}

// ClearAndWrite clears a tab then writes all rows (used for report_output).
func (c *Client) ClearAndWrite(tab string, rows [][]any) error {
	_, err := c.srv.Spreadsheets.Values.
		Clear(c.sheetID, tab, &sheets.ClearValuesRequest{}).
		Do()

	if err != nil {
		return fmt.Errorf("clear %s: %w", tab, err)
	}

	vr := &sheets.ValueRange{Values: rows}
	_, err = c.srv.Spreadsheets.Values.
		Update(c.sheetID, tab+"!A1", vr).
		ValueInputOption("USER_ENTERED").
		Do()

	if err != nil {
		return fmt.Errorf("write %s: %w", tab, err)
	}

	return nil
}

func (c *Client) GetReportValue(key string) (string, error) {
	rows, err := c.ReadTab("report_output")
	if err != nil {
		return "", err
	}

	for _, row := range rows {
		if len(row) >= 2 && row[0] == key {
			return row[1], nil
		}
	}

	return "", fmt.Errorf("key %q not found in report_output", key)
}


func SafeGetValue(row []string, idx map[string]int, key string) string {
	i, ok := idx[key]

	if !ok || i >= len(row) {
		return ""
	}

	return strings.TrimSpace(row[i])
}
