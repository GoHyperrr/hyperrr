package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var osExit = os.Exit

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		osExit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: coverage-check <coverage-file> <threshold>")
	}

	file := args[1]
	thresholdStr := args[2]
	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return fmt.Errorf("invalid threshold: %v", err)
	}

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("error opening coverage file: %v", err)
	}
	defer f.Close()

	coverage, err := calculateCoverage(f)
	if err != nil {
		return fmt.Errorf("error calculating coverage: %v", err)
	}

	fmt.Fprintf(out, "Total coverage: %.2f%%\n", coverage)

	if coverage < threshold {
		return fmt.Errorf("coverage below threshold: %.2f%% < %.2f%%", coverage, threshold)
	}

	return nil
}

func calculateCoverage(r io.Reader) (float64, error) {
	var totalStatements, coveredStatements int64
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		// mode line
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		statements, err1 := strconv.ParseInt(parts[len(parts)-2], 10, 64)
		count, err2 := strconv.ParseInt(parts[len(parts)-1], 10, 64)

		if err1 == nil && err2 == nil {
			totalStatements += statements
			if count > 0 {
				coveredStatements += statements
			}
		}
	}

	if totalStatements == 0 {
		return 100.0, nil
	}

	return (float64(coveredStatements) / float64(totalStatements)) * 100, nil
}
