package scanner

import (
	"github.com/dsecuredcom/xssscan/internal/payload"
)

type Job struct {
	URL        string
	Parameters []string
	Payloads   []payload.Payload
	Method     string
}
