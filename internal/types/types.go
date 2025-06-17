package types

type Result struct {
	URL        string
	Parameter  string
	Payload    string
	Reflected  bool
	StatusCode int
	Error      error
}
