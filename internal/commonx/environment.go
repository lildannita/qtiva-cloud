package commonx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

// Читает обязательную переменную окружения
func RequireString(key string) (string, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return "", fmt.Errorf("переменная окружения %s не задана", key)
	}
	return v, nil
}

// Читает обязательную int-переменную окружения
func RequireInt(key string) (int, error) {
	s, err := RequireString(key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("переменная %s должна быть целым числом: %w", key, err)
	}
	return n, nil
}

// Читает обязательную int64-переменную окружения
func RequireInt64(key string) (int64, error) {
	s, err := RequireString(key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("переменная %s должна быть int64: %w", key, err)
	}
	return n, nil
}

// Читает обязательную duration-переменную окружения (например, 30s, 5m)
func RequireDuration(key string) (time.Duration, error) {
	s, err := RequireString(key)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("переменная %s должна быть duration (например 30s/5m): %w", key, err)
	}
	return d, nil
}
