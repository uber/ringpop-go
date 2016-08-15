package swim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var validateLabelTests = []struct {
	key   string
	value string
	err   error
}{
	{"hello", "world", nil},
	{"hello", "world=", nil},
	{"hello", "wor=ld", nil},
	{"hello", "=world", nil},
	{"hello=", "world", ErrLabelInvalidKey},
	{"hel=lo", "world", ErrLabelInvalidKey},
	{"=hello", "world", ErrLabelInvalidKey},
}

func TestValidateLabel(t *testing.T) {
	for _, test := range validateLabelTests {
		assert.Equal(t, test.err, validateLabel(test.key, test.value), "key: %q value: %q expected: %+v", test.key, test.value, test.err)
	}
}
