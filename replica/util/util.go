package util

import "math/rand"

// SelectInt takes an option and a default value and returns the default value if
// the option is equal to zero, and the option otherwise.
func SelectInt(opt, def int) int {
	if opt == 0 {
		return def
	}
	return opt
}

// Shuffle takes a slice of strings and returns a new slice containing the same
// strings in a random order
func Shuffle(strings []string) []string {
	newStrings := make([]string, len(strings))
	newIndexes := rand.Perm(len(strings))

	for o, n := range newIndexes {
		newStrings[n] = strings[o]
	}

	return newStrings
}
