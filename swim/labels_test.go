package swim

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

var isInternalLabelTests = []struct {
	key      string
	internal bool
}{
	{"hello", false},
	{"__hello", true},
}

func TestIsInternalLabel(t *testing.T) {
	for _, test := range isInternalLabelTests {
		assert.Equal(t, test.internal, isInternalLabel(test.key), test)
	}
}
