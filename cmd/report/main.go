package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"stockfolio/src/notify"
	"stockfolio/src/report"
	"stockfolio/src/storage"
)

var logger *zap.Logger

func main() {
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()

	if err := godotenv.Load(".env"); err != nil {
		logger.Fatal("load .env", zap.Error(err))
	}

	exe, err := os.Executable()
	if err != nil {
		logger.Fatal("get executable path", zap.Error(err))
	}

	historyDir := filepath.Join(filepath.Dir(exe), "history")
	currentFile := "week_" + time.Now().Format("2006-01-02") + ".json"

	logger.Info("stockfolioloading previous snapshot...")

	prev, err := report.LoadLatestSnapshot(historyDir, currentFile)
	if err != nil {
		logger.Warn("could not load previous snapshot", zap.Error(err))
	}

	logger.Info("stockfolioconnecting to GSheet...")

	gs, err := storage.New()
	if err != nil {
		logger.Fatal("gsheet", zap.Error(err))
	}

	logger.Info("stockfoliobuilding report...")

	r, err := report.Build(gs)
	if err != nil {
		logger.Fatal("build report", zap.Error(err))
	}

	logger.Info("stockfoliosaving snapshot...")

	if err := report.SaveSnapshot(historyDir, r); err != nil {
		logger.Warn("could not save snapshot", zap.Error(err))
	}

	logger.Info("stockfolioformatting...")

	text := notify.FormatDiscordMessage(r, prev)

	logger.Info("stockfoliowriting to report_output tab...")

	if err := report.WriteReportOutput(gs, r, text); err != nil {
		logger.Fatal("write output", zap.Error(err))
	}

	fmt.Println(text)

	if strings.ToLower(os.Getenv("DISCORD_ENABLED")) == "true" {
		webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")

		if webhookURL == "" {
			logger.Warn("DISCORD_ENABLED=true but DISCORD_WEBHOOK_URL is not set — skipping")
		} else {
			logger.Info("stockfoliosending to Discord...")

			if err := notify.SendDiscordMessage(webhookURL, text); err != nil {
				logger.Warn("discord send failed", zap.Error(err))
			} else {
				logger.Info("✓ Discord message sent")
			}
		}
	} else {
		logger.Info("Discord disabled", zap.String("hint", "set DISCORD_ENABLED=true to enable"))
	}

	logger.Info("✓ report complete")
}
