package swim

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

var isPrivateLabelTests = []struct {
	key     string
	private bool
}{
	{"hello", false},
	{"__hello", true},
}

func TestIsPrivateLabel(t *testing.T) {
	for _, test := range isPrivateLabelTests {
		assert.Equal(t, test.private, isPrivateLabel(test.key), test)
	}
}
