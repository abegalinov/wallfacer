package runner

// truncate returns s truncated to n bytes, with "..." appended if truncation occurred.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
