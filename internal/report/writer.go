package report

import (
	"fmt"
	"sort"

	"github.com/dsecuredcom/xssscan/internal/types"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

// WriteConsoleReport now only writes a summary since individual findings are printed immediately
func WriteConsoleReport(results []types.Result) {
	// Filter for only reflected results
	var reflectedResults []types.Result
	for _, result := range results {
		if result.Reflected {
			reflectedResults = append(reflectedResults, result)
		}
	}

	if len(reflectedResults) == 0 {
		fmt.Println("[!] No XSS reflections found")
		return
	}

	// Sort results by URL, then by parameter
	sort.Slice(reflectedResults, func(i, j int) bool {
		if reflectedResults[i].URL != reflectedResults[j].URL {
			return reflectedResults[i].URL < reflectedResults[j].URL
		}
		return reflectedResults[i].Parameter < reflectedResults[j].Parameter
	})

	fmt.Printf("\n%s📋 REFLECTION SUMMARY%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s===================%s\n", ColorCyan, ColorReset)

	// Group by URL for cleaner summary
	urlGroups := make(map[string][]types.Result)
	for _, result := range reflectedResults {
		urlGroups[result.URL] = append(urlGroups[result.URL], result)
	}

	for url, urlResults := range urlGroups {
		fmt.Printf("\n%s%s%s (%d reflections)\n", ColorBlue, url, ColorReset, len(urlResults))
		for _, result := range urlResults {
			fmt.Printf("  └─ %s%s%s: %s\n", ColorYellow, result.Parameter, ColorReset, result.Payload)
		}
	}

	fmt.Printf("\n%sTotal reflections found: %d%s\n", ColorRed, len(reflectedResults), ColorReset)
	fmt.Printf("Please verify these findings manually.\n")
}
