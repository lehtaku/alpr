package main

import (
	"os"

	"github.com/lehtaku/alpr/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
