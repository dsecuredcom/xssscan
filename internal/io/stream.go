package io

import (
	"bufio"
	"context"
	"net/url"
	"os"
	"strings"
)

// StreamPaths liefert jede gültige URL sofort über einen Channel,
// statt alles vorab in den RAM zu laden.
func StreamPaths(ctx context.Context, filename string, out chan<- string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	go func() {
		defer close(out)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if _, err := url.Parse(line); err != nil {
				continue // ungültige Zeilen überspringen
			}
			select {
			case out <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
