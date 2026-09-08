package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"stockfolio/src/app"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	if err := godotenv.Load(".env"); err != nil {
		logger.Fatal("load .env", zap.Error(err))
	}

	text, err := app.Run(logger)
	if err != nil {
		logger.Fatal("report", zap.Error(err))
	}

	fmt.Println(text)
}
