package proxy

// firstNonEmpty returns the first non-empty string from a list of values.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
