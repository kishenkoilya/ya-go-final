package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	encoded := base64.RawStdEncoding.EncodeToString(append(salt, hash...))
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s", encoded), nil
}

func CheckPassword(password, encodedHash string) (bool, error) {
	sugar.Infoln(encodedHash)
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 {
		return false, fmt.Errorf("invalid hash format")
	}
	actualHashData := parts[4]

	bytes, err := base64.RawStdEncoding.DecodeString(actualHashData)
	if err != nil {
		return false, err
	}

	salt, hash := bytes[:16], bytes[16:]

	comparisonHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	sugar.Infoln(comparisonHash)
	return subtle.ConstantTimeCompare(hash, comparisonHash) == 1, nil
}

func CheckLuhn(number string) (bool, error) {
	var sum int
	sugar.Infoln(number)
	n, err := strconv.Atoi(number)
	if err != nil {
		sugar.Errorln("Error converting digits in number.")
		return false, err
	}
	for i := 0; n > 0; i++ {
		digit := n % 10
		if i%2 != 0 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		n /= 10
		sum += digit
	}
	return sum%10 == 0, nil
}
