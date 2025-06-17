package scanner

import (
	"bytes"
	"github.com/dsecuredcom/xssscan/internal/payload"
)

func checkReflections(body []byte, payloads []payload.Payload) map[string]bool {
	reflections := make(map[string]bool, len(payloads))
	for _, p := range payloads {
		reflections[p.Value] = bytes.Contains(body, []byte(p.Value))
	}
	return reflections
}
