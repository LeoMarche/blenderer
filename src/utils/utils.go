package utils

//IsIn checks if a string is in a array of string
func IsIn(s string, t []string) int {

	for i := 0; i < len(t); i++ {
		if t[i] == s {
			return i
		}
	}

	return -1
}
