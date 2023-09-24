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
