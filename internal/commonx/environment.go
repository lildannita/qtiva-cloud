package commonx

import (
	"os"

	"github.com/joho/godotenv"
)

// Загружаем .env только в dev-режиме
func LoadDotenvIfDev() {
	if os.Getenv("QTIVA_ENV") == "prod" {
		return
	}
	// Ошибку игнорируем: .env может не существовать
	_ = godotenv.Load()
}
