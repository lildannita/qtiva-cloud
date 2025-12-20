package authx

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Хеширует пароль bcrypt'ом
func HashPassword(password string) (string, error) {
	const cost = 12
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("authx: не удалось захешировать пароль: %w", err)
	}
	return string(hash), nil
}

// Проверяет соответствие пароля bcrypt-хешу
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
