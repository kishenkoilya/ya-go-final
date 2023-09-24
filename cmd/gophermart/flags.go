package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	Address              string `env:"RUN_ADDRESS"`
	DatabaseURI          string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func getVars() *Config {
	address := flag.String("a", "", "An address the server will be running on")
	databaseUri := flag.String("d", "", "An address database is located at")
	accrualSystemAddress := flag.String("r", "", "An address of accrual system")
	// postgresql://postgres:gpadmin@localhost:5432/postgres?sslmode=disable
	flag.Parse()

	var cfg Config
	error := env.Parse(&cfg)
	if error != nil {
		log.Fatal(error)
	}
	if cfg.Address == "" {
		cfg.Address = *address
	}
	if cfg.DatabaseURI == "" {
		cfg.DatabaseURI = *databaseUri
	}
	if cfg.AccrualSystemAddress == "" {
		cfg.AccrualSystemAddress = *accrualSystemAddress
	}
	return &cfg
}

func (conf *Config) printConfig() {
	fmt.Printf("Address: %s; Database Uri: %s; Accrual System Address: %s;\n",
		conf.Address, conf.DatabaseURI, conf.AccrualSystemAddress)
}
