//go:build ignore
package main

import (
	"fmt"
	"os"

	"github.com/GoHyperrr/hyperrr/internal/builder"
)

func main() {
	if err := builder.RunBuild(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}
}
