package payload

import (
	"crypto/md5"
	"fmt"
)

type Payload struct {
	Parameter string
	Value     string
}

// GeneratePayloads creates unique payloads for each parameter
func GeneratePayloads(parameters []string) []Payload {
	var payloads []Payload

	for _, param := range parameters {
		// Generate MD5 hash of parameter name
		hash := fmt.Sprintf("%x", md5.Sum([]byte(param)))
		first3 := hash[:3]
		last2 := hash[len(hash)-2:]

		// Create two payload variants
		payloads = append(payloads, Payload{
			Parameter: param,
			Value:     first3 + "\">" + last2,
		})
		payloads = append(payloads, Payload{
			Parameter: param,
			Value:     first3 + "'>" + last2,
		})
	}

	return payloads
}
