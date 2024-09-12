package utils

func Defaults(s string, def string) string {
	if s == "" {
		return def
	}
	return s
}

func PrependBytes(prefix, data []byte) []byte {
	out := make([]byte, len(prefix)+len(data))
	copy(out, prefix)
	copy(out[len(prefix):], data)
	return out
}

func JoinBytes2(prefix byte, data ...[]byte) []byte {
	out := []byte{prefix}
	for _, d := range data {
		out = append(out, d...)
	}
	return out
}
