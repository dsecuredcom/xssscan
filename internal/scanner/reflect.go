package scanner

import (
	"strings"

	"github.com/dsecuredcom/xssscan/internal/payload"
)

func checkReflections(body string, payloads []payload.Payload) map[string]bool {
	reflections := make(map[string]bool)

	for _, p := range payloads {
		reflections[p.Value] = strings.Contains(body, p.Value)
	}

	return reflections
}
