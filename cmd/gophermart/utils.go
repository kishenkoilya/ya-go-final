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
	// var sum int
	// for n, symb := range number {
	// 	digitStr := string(symb)
	// 	digit, err := strconv.Atoi(digitStr)
	// 	if err != nil {
	// 		sugar.Errorln("Error converting digits in number.")
	// 		return false, err
	// 	}
	// 	if n%2 != 0 {
	// 		digit *= 2
	// 		if digit > 9 {
	// 			digit -= 9
	// 		}
	// 	}
	// 	fmt.Println(digit)
	// 	fmt.Println(sum)
	// 	sum += digit
	// }
	res, err := luhnCheckDigit(number)
	return res%10 == 0, err
}

func luhnCheckDigit(s string) (int, error) {
	number, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	fmt.Println(number)
	checkNumber := luhnChecksum(number)

	if checkNumber == 0 {
		return 0, nil
	}
	return 10 - checkNumber, nil
}

func luhnChecksum(number int) int {
	var luhn int

	for i := 0; number > 0; i++ {
		cur := number % 10

		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}
		fmt.Println(number)
		fmt.Println(cur)
		fmt.Println(luhn)
		luhn += cur
		number = number / 10
	}
	return luhn % 10
}
