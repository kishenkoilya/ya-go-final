package main

import (
	"go.uber.org/zap"
)

var sugar zap.SugaredLogger

func main() {
	config := getVars()
	config.printConfig()
	runServer(config)
}
