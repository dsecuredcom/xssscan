package scanner

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/dsecuredcom/xssscan/internal/payload"
	"github.com/dsecuredcom/xssscan/internal/util"
)

// Add color constants for immediate reporting
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

type Config struct {
	Method      string
	Concurrency int
	Workers     int
	Retries     int
	HTTPClient  *util.HTTPClient
	Verbose     bool
}

func Run(ctx context.Context, config Config, paths <-chan string, batches [][]string) error {
	// Create rate limiter
	limiter := rate.NewLimiter(rate.Limit(config.Concurrency), config.Concurrency)

	// Create job queue
	jobs := make(chan Job, config.Concurrency*2)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go worker(ctx, &wg, jobs, limiter, config)
	}

	// Generate and send jobs
	go func() {
		defer close(jobs)

		for path := range paths {
			for _, batch := range batches {
				// Generate payloads for this batch
				allPayloads := payload.GeneratePayloads(batch)

				// Group payloads by variant type
				doubleQuotePayloads := make([]payload.Payload, 0)
				singleQuotePayloads := make([]payload.Payload, 0)

				for _, p := range allPayloads {
					if strings.Contains(p.Value, "\">") {
						doubleQuotePayloads = append(doubleQuotePayloads, p)
					} else if strings.Contains(p.Value, "'>") {
						singleQuotePayloads = append(singleQuotePayloads, p)
					}
				}

				// Create job for double quote variant (">)
				if len(doubleQuotePayloads) > 0 {
					job := Job{
						URL:        path,
						Parameters: batch,
						Payloads:   doubleQuotePayloads,
						Method:     config.Method,
					}

					select {
					case jobs <- job:
					case <-ctx.Done():
						return
					}
				}

				// Create job for single quote variant ('>)
				if len(singleQuotePayloads) > 0 {
					job := Job{
						URL:        path,
						Parameters: batch,
						Payloads:   singleQuotePayloads,
						Method:     config.Method,
					}

					select {
					case jobs <- job:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	return nil
}

func worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan Job, limiter *rate.Limiter, config Config) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}

			// Wait for rate limiter
			if err := limiter.Wait(ctx); err != nil {
				return
			}

			processJob(ctx, job, config)

		case <-ctx.Done():
			return
		}
	}
}

func processJob(ctx context.Context, job Job, config Config) {
	// Create request with payloads
	resp, err := config.HTTPClient.Request(ctx, job.Method, job.URL, job.Payloads)

	if err != nil {
		// Show error in verbose mode only
		if config.Verbose {
			fmt.Printf("[%sERROR%s] %s %s - %v\n", ColorRed, ColorReset, job.Method, job.URL, err)
		}
		return
	}

	// Show verbose output AFTER getting response - URL and status together
	if config.Verbose {
		statusColor := ColorGreen
		if resp.StatusCode >= 400 {
			statusColor = ColorRed
		} else if resp.StatusCode >= 300 {
			statusColor = ColorYellow
		}

		if job.Method == "GET" {
			// For GET requests, build and show the complete URL with all parameters
			u, err := url.Parse(job.URL)
			if err == nil {
				q := u.Query()
				for _, p := range job.Payloads {
					q.Set(p.Parameter, p.Value)
				}
				u.RawQuery = q.Encode()
				fmt.Printf("[%s%d%s] %s %s\n", statusColor, resp.StatusCode, ColorReset, job.Method, u.String())
			}
		} else {
			// For POST requests, show URL and all form data
			fmt.Printf("[%s%d%s] %s %s\n", statusColor, resp.StatusCode, ColorReset, job.Method, job.URL)
			var formPairs []string
			for _, p := range job.Payloads {
				formPairs = append(formPairs, fmt.Sprintf("%s=%s", p.Parameter, p.Value))
			}
			fmt.Printf("    Body: %s\n", strings.Join(formPairs, "&"))
		}
	}

	// Check for reflections
	reflections := checkReflections(resp.Body, job.Payloads)

	// Print immediately if reflected - NO STORAGE
	for _, p := range job.Payloads {
		if reflections[p.Value] {
			if job.Method == "GET" {
				// For GET requests, show the full URL with parameters
				u, err := url.Parse(job.URL)
				if err == nil {
					q := u.Query()
					q.Set(p.Parameter, p.Value)
					u.RawQuery = q.Encode()
					fmt.Printf("[%sREFLECTED%s] [%sGET%s] %s\n", ColorRed, ColorReset, ColorGreen, ColorReset, u.String())
				}
			} else {
				// For POST requests, show URL and payload on separate line
				fmt.Printf("[%sREFLECTED%s] [%sPOST%s] %s\n", ColorRed, ColorReset, ColorGreen, ColorReset, job.URL)
				fmt.Printf("%s=%s\n", p.Parameter, p.Value)
			}
		}
	}
}
