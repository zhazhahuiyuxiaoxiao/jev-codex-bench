package cachettl

import "strings"

func NormalizeKey(key string) string { return strings.TrimSpace(key) }
