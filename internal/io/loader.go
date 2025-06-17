package io

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// LoadPaths reads and validates URLs from a file
func LoadPaths(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var paths []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate URL
		if _, err := url.Parse(line); err != nil {
			return nil, fmt.Errorf("invalid URL at line %d: %s", lineNum, line)
		}

		paths = append(paths, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no valid paths found in file")
	}

	return paths, nil
}

// LoadParameters reads parameter names from a file
func LoadParameters(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var parameters []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate parameter name (basic check)
		if strings.Contains(line, "=") || strings.Contains(line, "&") {
			return nil, fmt.Errorf("invalid parameter name: %s (should not contain = or &)", line)
		}

		parameters = append(parameters, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if len(parameters) == 0 {
		return nil, fmt.Errorf("no valid parameters found in file")
	}

	return parameters, nil
}
