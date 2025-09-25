package main

import (
	"fmt"
	"os"

	"docker-impl/pkg/cli"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		Level:         logrus.InfoLevel,
	})

	app, err := cli.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create CLI app: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}