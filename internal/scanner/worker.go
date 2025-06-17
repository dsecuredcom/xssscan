// internal/scanner/worker.go
package scanner

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/dsecuredcom/xssscan/internal/payload"
	"github.com/dsecuredcom/xssscan/internal/report"
	"github.com/dsecuredcom/xssscan/internal/types"
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
	Reporter    *report.Collector
	Verbose     bool
}

// FIXED: Test each batch with both payload variants
func Run(ctx context.Context, config Config, paths []string, batches [][]string) error {
	// Create rate limiter
	limiter := rate.NewLimiter(rate.Limit(config.Concurrency), config.Concurrency)

	// Create job queue
	jobs := make(chan Job, 1000)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go worker(ctx, &wg, jobs, limiter, config)
	}

	// Generate and send jobs - FIXED: Generate both payload variants per batch
	go func() {
		defer close(jobs)

		for _, path := range paths {
			for _, batch := range batches {
				// Generate payloads for this batch - this creates both "> and '> variants
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
		// Report error for all payloads in this job
		for _, p := range job.Payloads {
			config.Reporter.AddResult(types.Result{
				URL:       job.URL,
				Parameter: p.Parameter,
				Payload:   p.Value,
				Reflected: false,
				Error:     err,
			})
		}

		// Show error in verbose mode
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

	// Report results and print immediately if reflected
	for _, p := range job.Payloads {
		reflected := reflections[p.Value]

		// Store result
		config.Reporter.AddResult(types.Result{
			URL:        job.URL,
			Parameter:  p.Parameter,
			Payload:    p.Value,
			Reflected:  reflected,
			StatusCode: resp.StatusCode,
		})

		// Print reflection immediately when found with optimized format (always shown)
		if reflected {
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
