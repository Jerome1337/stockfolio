package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"stockfolio/src/helpers"
	"stockfolio/src/report"
	"stockfolio/src/storage"
)

func FormatDiscordMessage(r *report.Report, prev *report.Snapshot) string {
	var b strings.Builder

	line := func(s string) {
		b.WriteString(s);
		b.WriteString("\n")
	}
	blank := func() { b.WriteString("\n") }

	line(fmt.Sprintf("📊 **PORTFOLIO** — %s", r.Date.Format("02 Jan 2006")))
	blank()

	// Summary — one code block, compact
	line("```")
	line(fmt.Sprintf("Invested   %s", helpers.FormatEUR(r.TotalInvestedEUR)))
	line(fmt.Sprintf("Value      %s  (%s)", helpers.FormatEUR(r.TotalValueEUR), helpers.FormatPct(r.TotalPnLPct)))
	line(fmt.Sprintf("S&P week   %s", helpers.FormatPct(r.MarketWeekPct)))
	line(fmt.Sprintf("Divs YTD   %s", helpers.FormatEUR(r.DividendsYTD)))
	line("```")
	blank()

	// Week-over-week diff
	if prev != nil {
		line(fmt.Sprintf("📅 **vs last week** (%s)", prev.Date))
		line("```")

		valueDiffPct := 0.0
		if prev.TotalValueEUR > 0 {
			valueDiffPct = helpers.Round2((r.TotalValueEUR-prev.TotalValueEUR) / prev.TotalValueEUR * 100)
		}
		pnlDiff := helpers.Round2(r.TotalPnLPct - prev.TotalPnLPct)

		line(fmt.Sprintf("Value  %s → %s  (%s)", helpers.FormatEUR(prev.TotalValueEUR), helpers.FormatEUR(r.TotalValueEUR), helpers.FormatPct(valueDiffPct)))
		line(fmt.Sprintf("PnL    %s → %s  (%spp)", helpers.FormatPct(prev.TotalPnLPct), helpers.FormatPct(r.TotalPnLPct), helpers.FormatPct(pnlDiff)))

		type mover struct {
			pos  storage.Position
			prev float64
			curr float64
			diff float64
		}

		var movers []mover

		for _, p := range r.Positions {
			if prevPnL, ok := prev.Positions[p.Ticker]; ok {
				diff := helpers.Round2(p.PnLPct - prevPnL)
				if math.Abs(diff) >= 1.0 {
					movers = append(movers, mover{p, prevPnL, p.PnLPct, diff})
				}
			}
		}

		sort.Slice(movers, func(i, j int) bool {
			return math.Abs(movers[i].diff) > math.Abs(movers[j].diff)
		})

		if len(movers) > 5 {
			movers = movers[:5]
		}

		if len(movers) > 0 {
			line("")
			for _, m := range movers {
				line(fmt.Sprintf("%-22s %s → %s  (%spp)", report.DisplayName(m.pos), helpers.FormatPct(m.prev), helpers.FormatPct(m.curr), helpers.FormatPct(m.diff)))
			}
		}

		line("```")
		blank()
	}

	// ETF sleeve
	if len(r.ETFs) > 0 {
		etfs := report.SortedByPnL(r.ETFs)
		line("📦 **ETFs**")
		line("```")
		for _, p := range etfs {
			name := report.DisplayName(p)
			delta := ""
			if p.BenchmarkTicker != "" {
				diff := p.PnLPct - p.BenchmarkPnLPct
				if diff >= 0 {
					delta = fmt.Sprintf(" · vs bench +%.1f%%", diff)
				} else {
					delta = fmt.Sprintf(" · vs bench %.1f%%", diff)
				}
			}
			line(fmt.Sprintf("%-22s  %-10s  %s%s", name, helpers.FormatEUR(p.ValueEUR), helpers.FormatPct(p.PnLPct), delta))
		}
		line("```")
		blank()
	}

	// Stock sleeve
	if len(r.Stocks) > 0 {
		stocks := report.SortedByPnL(r.Stocks)
		line("📈 **Stocks**")
		line("```")
		for _, p := range stocks {
			name := report.DisplayName(p)
			yoc := ""
			if p.YieldOnCost > 0 {
				yoc = fmt.Sprintf(" · %.1f%% yoc", p.YieldOnCost)
			}
			flag := ""
			if p.Flag == "REVIEW" {
				flag = " ⚠"
			} else if p.Flag == "WATCH" {
				flag = " 👁"
			}
			line(fmt.Sprintf("%-22s  %-10s  %s%s%s", name, helpers.FormatEUR(p.ValueEUR), helpers.FormatPct(p.PnLPct), yoc, flag))
		}
		line("```")
		blank()
	}

	// Flags — one block per flag, clearly separated
	if len(r.Flags) > 0 {
		line("⚠️ **Flags**")
		for _, p := range r.Flags {
			icon := "👁"
			if p.Flag == "REVIEW" {
				icon = "🔴"
			}
			line(fmt.Sprintf("%s **%s** — %s", icon, report.DisplayName(p), p.FlagReason))
		}
		blank()
	} else {
		line("✅ No flags this week")
		blank()
	}

	// Dividends reminder only if something landed this month
	if r.DividendsMTD > 0 {
		line(fmt.Sprintf("💰 %s received this month — reinvest?", helpers.FormatEUR(r.DividendsMTD)))
	}

	return b.String()
}


// SendDiscordMessage posts the report text to a Discord webhook.
// Discord has a 2000 char limit per message, splits if needed.
func SendDiscordMessage(webhookURL, text string) error {
	chunks := splitDiscordMessage(text, 2000)

	for _, chunk := range chunks {
		if err := postDiscordMessage(webhookURL, chunk); err != nil {
			return err
		}
	}

	return nil
}

func postDiscordMessage(webhookURL, content string) error {
	payload, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// splitDiscordMessage splits a message into chunks of maxLen chars, breaking on newlines to avoid cutting mid-line.
func splitDiscordMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	var current strings.Builder

	for _, line := range lines {
		// +1 for the newline we'll add
		if current.Len()+len(line)+1 > maxLen {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(line);
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}
