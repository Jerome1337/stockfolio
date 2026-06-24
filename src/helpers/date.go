package helpers

import (
	"fmt"
	"time"
)

func FlexParseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-01-2006",
		"02/01/2006",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}
