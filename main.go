package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/dsecuredcom/xssscan/internal/report"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dsecuredcom/xssscan/internal/batcher"
	"github.com/dsecuredcom/xssscan/internal/io"
	"github.com/dsecuredcom/xssscan/internal/scanner"
	"github.com/dsecuredcom/xssscan/internal/util"
)

type Config struct {
	PathsFile      string
	ParametersFile string
	Method         string
	Concurrency    int
	ParameterBatch int
	Timeout        time.Duration
	Proxy          string
	Workers        int
	Insecure       bool
	Retries        int
	Verbose        bool
}

func main() {
	config := &Config{}

	flag.StringVar(&config.PathsFile, "paths", "", "File with target URLs (one per line)")
	flag.StringVar(&config.ParametersFile, "parameters", "", "File with parameter names (one per line)")
	flag.StringVar(&config.Method, "method", "GET", "HTTP method (GET or POST)")
	flag.IntVar(&config.Concurrency, "concurrency", 20, "Max requests per second")
	flag.IntVar(&config.ParameterBatch, "parameter-batch", 5, "Number of parameters per request")
	flag.DurationVar(&config.Timeout, "timeout", 15*time.Second, "Client timeout per request")
	flag.StringVar(&config.Proxy, "proxy", "", "Optional upstream proxy")
	flag.IntVar(&config.Workers, "workers", 0, "Number of workers (default: concurrency)")
	flag.BoolVar(&config.Insecure, "insecure", false, "Ignore TLS certificate errors")
	flag.IntVar(&config.Retries, "retries", 0, "Number of retries on failure")
	flag.BoolVar(&config.Verbose, "verbose", false, "Show all requests and HTTP status codes")

	flag.Parse()

	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	if config.Workers == 0 {
		config.Workers = config.Concurrency
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[!] Received interrupt signal, shutting down gracefully...")
		cancel()
	}()

	if err := run(ctx, config); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}
}

func validateConfig(config *Config) error {
	if config.PathsFile == "" {
		return fmt.Errorf("paths file is required")
	}
	if config.ParametersFile == "" {
		return fmt.Errorf("parameters file is required")
	}
	if config.Method != "GET" && config.Method != "POST" {
		return fmt.Errorf("method must be GET or POST")
	}
	if config.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	if config.ParameterBatch <= 0 {
		return fmt.Errorf("parameter-batch must be positive")
	}
	return nil
}

func run(ctx context.Context, config *Config) error {
	// Load input files
	fmt.Print("[+] Loading input files...")
	paths, err := io.LoadPaths(config.PathsFile)
	if err != nil {
		return fmt.Errorf("loading paths: %w", err)
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(paths), func(i, j int) {
		paths[i], paths[j] = paths[j], paths[i]
	})

	parameters, err := io.LoadParameters(config.ParametersFile)
	if err != nil {
		return fmt.Errorf("loading parameters: %w", err)
	}
	fmt.Printf(" Done\n")

	// Create parameter batches
	batches := batcher.CreateBatches(parameters, config.ParameterBatch)

	pathsCh := make(chan string, 128) // kleiner Puffer
	go func() {
		defer close(pathsCh)
		for _, p := range paths {
			select {
			case pathsCh <- p: // weitergeben
			case <-ctx.Done():
				return // sauber beenden, falls Ctrl-C
			}
		}
	}()

	// Calculate total HTTP requests
	totalHTTPRequests := len(paths) * len(batches) * 2

	rep := report.New(totalHTTPRequests)
	defer rep.Close()

	fmt.Printf("[+] Loaded:\n")
	fmt.Printf("    • %d paths\n", len(paths))
	fmt.Printf("    • %d parameters\n", len(parameters))
	fmt.Printf("    • %d chunks (parameters/chunk size: %d/%d)\n", len(batches), len(parameters), config.ParameterBatch)
	fmt.Printf("    • %d HTTP requests total (%d paths × %d chunks × 2 variants)\n",
		totalHTTPRequests, len(paths), len(batches))

	// Initialize HTTP client
	httpClient := util.NewHTTPClient(util.HTTPConfig{
		Timeout:  config.Timeout,
		Proxy:    config.Proxy,
		Insecure: config.Insecure,
		MaxConns: config.Workers,
	})

	// Create scanner configuration - NO REPORTER
	scanConfig := scanner.Config{
		Method:      config.Method,
		Concurrency: config.Concurrency,
		Workers:     config.Workers,
		Retries:     config.Retries,
		HTTPClient:  httpClient,
		Verbose:     config.Verbose,
		Reporter:    rep,
	}

	// Start scanning
	fmt.Printf("[+] Starting %d RPS with %d workers...\n", config.Concurrency, config.Workers)
	if config.Verbose {
		fmt.Printf("[+] Verbose mode enabled - showing all requests\n")
	}
	fmt.Printf("[+] Reflections will be reported immediately as found:\n")

	if err := scanner.Run(ctx, scanConfig, pathsCh, batches); err != nil {
		return fmt.Errorf("scanning failed: %w", err)
	}

	// Simple completion message - no summary
	fmt.Printf("\n[+] Scan completed.\n")

	return nil
}
