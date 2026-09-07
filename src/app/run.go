// Package app holds the report pipeline shared by the CLI and the HTTP handler.
package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"stockfolio/src/notify"
	"stockfolio/src/report"
	"stockfolio/src/storage"
)

// Run builds the report, saves the snapshot, writes it to the sheet and, when
// enabled, posts it to Discord. It returns the formatted report text.
func Run(logger *zap.Logger) (string, error) {
	historyDir := os.Getenv("HISTORY_DIR")
	if historyDir == "" {
		historyDir = "./history"
	}

	currentFile := "week_" + time.Now().Format("2006-01-02") + ".json"

	logger.Info("stockfolio loading previous snapshot...")

	prev, err := report.LoadLatestSnapshot(historyDir, currentFile)
	if err != nil {
		logger.Warn("load snapshot", zap.Error(err))
	}

	logger.Info("stockfolio connecting to GSheet...")

	gs, err := storage.New()
	if err != nil {
		return "", fmt.Errorf("gsheet: %w", err)
	}

	logger.Info("stockfolio building report...")

	rpt, err := report.Build(gs)
	if err != nil {
		return "", fmt.Errorf("build report: %w", err)
	}

	logger.Info("stockfolio saving snapshot...")

	if err := report.SaveSnapshot(historyDir, rpt); err != nil {
		logger.Warn("save snapshot", zap.Error(err))
	}

	logger.Info("stockfolio formatting...")

	text := notify.FormatDiscordMessage(rpt, prev)

	logger.Info("stockfolio writing to report_output tab...")

	if err := report.WriteReportOutput(gs, rpt, text); err != nil {
		return "", fmt.Errorf("write output: %w", err)
	}

	if strings.ToLower(os.Getenv("DISCORD_ENABLED")) == "true" {
		webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")

		if webhookURL == "" {
			logger.Warn("DISCORD_ENABLED=true but DISCORD_WEBHOOK_URL not set")
		} else if err := notify.SendDiscordMessage(webhookURL, text); err != nil {
			logger.Warn("discord send failed", zap.Error(err))
		}
	}

	logger.Info("✓ report complete")

	return text, nil
}
