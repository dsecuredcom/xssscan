package scanner

// Variant bestimmt, welche Quote-Variante wir in der Nutzlast brauchen.
type Variant uint8

const (
	VariantDoubleQuote Variant = iota //  ">
	VariantSingleQuote                //  '>
)

type Job struct {
	URL        string
	Parameters []string
	Variant    Variant
	Method     string
}
