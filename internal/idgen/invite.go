package idgen

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Алфавит для генерации invite-кодов (без похожих символов: 0/O, 1/I/L)
const inviteAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// Генерирует invite-код по шаблону INV-XXXX-YYYY
// где XXXX — случайная часть, YYYY — суффикс клиента (первые 4 символа client_id)
func GenerateInviteCode(clientID string) (string, error) {
	// Генерируем 4 случайных символа
	randomPart, err := randomString(4)
	if err != nil {
		return "", fmt.Errorf("idgen: не удалось сгенерировать случайную часть: %w", err)
	}

	// Берём суффикс из client_id (убираем префикс "cl_" и берём первые 4 символа)
	clientSuffix := strings.TrimPrefix(clientID, "cl_")
	if len(clientSuffix) > 4 {
		clientSuffix = clientSuffix[:4]
	}
	// Приводим к верхнему регистру для единообразия
	clientSuffix = strings.ToUpper(clientSuffix)

	return fmt.Sprintf("INV-%s-%s", randomPart, clientSuffix), nil
}

// Генерирует случайную строку заданной длины из inviteAlphabet
func randomString(length int) (string, error) {
	result := make([]byte, length)
	alphabetLen := big.NewInt(int64(len(inviteAlphabet)))

	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = inviteAlphabet[idx.Int64()]
	}

	return string(result), nil
}
