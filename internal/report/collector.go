package report

import (
	"sync"

	"github.com/dsecuredcom/xssscan/internal/types"
)

type Collector struct {
	mu      sync.RWMutex
	results []types.Result
}

func NewCollector() *Collector {
	return &Collector{
		results: make([]types.Result, 0),
	}
}

func (c *Collector) AddResult(result types.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, result)
}

func (c *Collector) GetResults() []types.Result {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	results := make([]types.Result, len(c.results))
	copy(results, c.results)
	return results
}
