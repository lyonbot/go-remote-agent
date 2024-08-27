package utils

func Defaults(s string, def string) string {
	if s == "" {
		return def
	}
	return s
}
