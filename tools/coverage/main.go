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
	type lineRange struct {
		file       string
		statements int64
	}
	
	// Map to track if a specific line range is covered
	// key: "file:startLine.startChar,endLine.endChar"
	coveredMap := make(map[string]bool)
	statementsMap := make(map[string]int64)

	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		// mode line
	}

	for scanner.Scan() {
		line := scanner.Text()
		
		// Skip generated and test files
		if strings.Contains(line, "generated.go") || strings.Contains(line, "models_gen.go") || strings.Contains(line, "_test.go") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// Parts format: <file>:<range> <statements> <count>
		rangePart := parts[0]
		statements, _ := strconv.ParseInt(parts[len(parts)-2], 10, 64)
		count, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)

		if count > 0 {
			coveredMap[rangePart] = true
		}
		statementsMap[rangePart] = statements
	}

	var totalStatements, coveredStatements int64
	for rangePart, statements := range statementsMap {
		totalStatements += statements
		if coveredMap[rangePart] {
			coveredStatements += statements
		}
	}

	if totalStatements == 0 {
		return 100.0, nil
	}

	return (float64(coveredStatements) / float64(totalStatements)) * 100, nil
}
