package helpers

import "fmt"

const EUR = "EUR";

func FormatEUR(v float64) string {
	return fmt.Sprintf("€%.2f", v)
}

func FormatPct(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+%.1f%%", v)
	}

	return fmt.Sprintf("%.1f%%", v)
}
