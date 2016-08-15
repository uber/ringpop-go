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

var countingLabelsTests = []struct {
	labels map[string]string
	count  int
}{
	{nil, 0},
	{map[string]string{}, 0},
	{map[string]string{"nonInternal": "hello"}, 1},
	{map[string]string{"__Internal": "world"}, 0},
	{map[string]string{"nonInternal": "hello", "__Internal": "world"}, 1},
}

func TestCountNonInternalLabels(t *testing.T) {
	for _, test := range countingLabelsTests {
		assert.Equal(t, test.count, countNonInternalLabels(test.labels))
	}
}
