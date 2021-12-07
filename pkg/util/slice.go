package util

// StringInSlice returns true if s is in list
func StringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}
