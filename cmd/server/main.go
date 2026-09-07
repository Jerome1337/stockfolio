package main

import (
	"encoding/json"
	"net/http"
	"os"
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

	// Optional: in a container the values come from the environment.
	if err := godotenv.Load(".env"); err != nil {
		logger.Warn("no .env file, using environment", zap.Error(err))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/stockfolio/report/generate", handleGenerate)

	logger.Info("listening", zap.String("addr", ":"+port))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.Fatal("server", zap.Error(err))
	}
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	historyDir := os.Getenv("HISTORY_DIR")
	if historyDir == "" {
		historyDir = "./history"
	}
	currentFile := "week_" + time.Now().Format("2006-01-02") + ".json"

	prev, err := report.LoadLatestSnapshot(historyDir, currentFile)
	if err != nil {
		logger.Warn("load snapshot", zap.Error(err))
	}

	gs, err := storage.New()
	if err != nil {
		logger.Error("gsheet", zap.Error(err))
		http.Error(w, "gsheet connection failed", http.StatusInternalServerError)
		return
	}

	rpt, err := report.Build(gs)
	if err != nil {
		logger.Error("build report", zap.Error(err))
		http.Error(w, "report build failed", http.StatusInternalServerError)
		return
	}

	if err := report.SaveSnapshot(historyDir, rpt); err != nil {
		logger.Warn("save snapshot", zap.Error(err))
	}

	text := notify.FormatDiscordMessage(rpt, prev)

	if err := report.WriteReportOutput(gs, rpt, text); err != nil {
		logger.Error("write output", zap.Error(err))
		http.Error(w, "write output failed", http.StatusInternalServerError)
		return
	}

	if strings.ToLower(os.Getenv("DISCORD_ENABLED")) == "true" {
		webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
		if webhookURL == "" {
			logger.Warn("DISCORD_ENABLED=true but DISCORD_WEBHOOK_URL not set")
		} else if err := notify.SendDiscordMessage(webhookURL, text); err != nil {
			logger.Warn("discord send failed", zap.Error(err))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"text": text})
}
