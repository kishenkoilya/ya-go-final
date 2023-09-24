package main

import (
	"fmt"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	fmt.Println(string(hashedPassword))
	return string(hashedPassword), nil
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
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
		if n%2 != 0 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		fmt.Println(digit)
		fmt.Println(sum)
		sum += digit
	}
	return sum%10 == 0, nil
}
