package main

import (
	"log"

	"github.com/GoHyperrr/hyperrr/internal/app"
)

var osExit = func(err error) {
	log.Fatal(err)
}

var appRun = app.Run

func main() {
	if err := run(); err != nil {
		osExit(err)
	}
}

func run() error {
	return appRun()
}
