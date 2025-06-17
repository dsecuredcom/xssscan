package scanner

import (
	"context"
	"fmt"
	"github.com/dsecuredcom/xssscan/internal/payload"
	"github.com/dsecuredcom/xssscan/internal/report"
	"golang.org/x/time/rate"
	"net/url"
	"strings"
	"sync"

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
	Reporter    *report.Reporter
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

				// 1.  Variante mit ">
				select {
				case jobs <- Job{
					URL:        path,
					Parameters: batch,
					Variant:    VariantDoubleQuote,
					Method:     config.Method,
				}:
				case <-ctx.Done():
					return
				}

				// 2.  Variante mit '>
				select {
				case jobs <- Job{
					URL:        path,
					Parameters: batch,
					Variant:    VariantSingleQuote,
					Method:     config.Method,
				}:
				case <-ctx.Done():
					return
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
	// ▸ Payloads erst JETZT bauen  (Back-Pressure)
	all := payload.GeneratePayloads(job.Parameters)

	payloads := make([]payload.Payload, 0, len(all)/2)
	for _, p := range all {
		switch job.Variant {
		case VariantDoubleQuote:
			if strings.Contains(p.Value, "\">") {
				payloads = append(payloads, p)
			}
		case VariantSingleQuote:
			if strings.Contains(p.Value, "'>") {
				payloads = append(payloads, p)
			}
		}
	}
	if len(payloads) == 0 {
		return
	}

	// HTTP-Request
	resp, err := config.HTTPClient.Request(ctx, job.Method, job.URL, payloads)
	if err != nil {
		if config.Verbose {
			fmt.Printf("[%sERROR%s] %s %s - %v\n", ColorRed, ColorReset, job.Method, job.URL, err)
		}
		config.Reporter.Inc(1)
		return
	}

	ct := strings.ToLower(resp.ContentType)
	if resp.ContentType != "" && !strings.Contains(ct, "html") { //   »text/html«, »text/html; charset=utf-8«, »application/xhtml+xml« …
		if config.Verbose {
			fmt.Printf("[skip] Non-HTML (%s) → %s %s\n", resp.ContentType, job.Method, job.URL)
		}
		config.Reporter.Inc(1)
		return
	}

	/* ---------- Verbose-Ausgabe (wie gehabt, nur payloads statt job.Payloads) ---------- */
	if config.Verbose {
		statusColor := ColorGreen
		if resp.StatusCode >= 400 {
			statusColor = ColorRed
		} else if resp.StatusCode >= 300 {
			statusColor = ColorYellow
		}

		if job.Method == "GET" {
			if u, err := url.Parse(job.URL); err == nil {
				q := u.Query()
				for _, p := range payloads {
					q.Set(p.Parameter, p.Value)
				}
				u.RawQuery = q.Encode()
				fmt.Printf("[%s%d%s] %s %s\n", statusColor, resp.StatusCode, ColorReset, job.Method, u.String())
			}
		} else {
			fmt.Printf("[%s%d%s] %s %s\n", statusColor, resp.StatusCode, ColorReset, job.Method, job.URL)
			var pairs []string
			for _, p := range payloads {
				pairs = append(pairs, fmt.Sprintf("%s=%s", p.Parameter, p.Value))
			}
			fmt.Printf("    Body: %s\n", strings.Join(pairs, "&"))
		}
	}

	/* ---------- Reflections sofort melden ---------- */
	reflections := checkReflections(resp.Body, payloads)
	for _, p := range payloads {
		if reflections[p.Value] {
			if job.Method == "GET" {
				if u, err := url.Parse(job.URL); err == nil {
					q := u.Query()
					q.Set(p.Parameter, p.Value)
					u.RawQuery = q.Encode()
					fmt.Printf("[%sREFLECTED%s] [%sGET%s] %s\n", ColorRed, ColorReset, ColorGreen, ColorReset, u.String())
				}
			} else {
				fmt.Printf("[%sREFLECTED%s] [%sPOST%s] %s\n", ColorRed, ColorReset, ColorGreen, ColorReset, job.URL)
				fmt.Printf("%s=%s\n", p.Parameter, p.Value)
			}
		}
	}

	if config.Reporter != nil {
		config.Reporter.Inc(1)
	}
}
