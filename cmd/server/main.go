package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"stockfolio/src/app"
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

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
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

	text, err := app.Run(logger)
	if err != nil {
		logger.Error("report", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"text": text})
}
