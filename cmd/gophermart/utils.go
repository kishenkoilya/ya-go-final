package main

import (
	"crypto/rand"
	"encoding/base64"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}

	hashWithSalt := append(salt, hashedPassword...)
	hashWithSaltBase64 := base64.StdEncoding.EncodeToString(hashWithSalt)

	return hashWithSaltBase64, string(salt), nil
}

func CheckPassword(password, salt, hash string) (bool, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}

	byteSalt := []byte(salt)
	hashWithSalt := append(byteSalt, hashedPassword...)
	hashWithSaltBase64 := base64.StdEncoding.EncodeToString(hashWithSalt)

	if hashWithSaltBase64 != hash {
		return false, nil
	}
	return true, nil
}

func CheckLuhn(number string) (bool, error) {
	var sum int
	for n, symb := range number {
		digitStr := string(symb)
		digit, err := strconv.Atoi(digitStr)
		if err != nil {
			sugar.Errorln("Error converting digits in number.")
			return false, err
		}
		if n%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0, nil
}
