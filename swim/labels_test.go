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

var validateLabelsTests = []struct {
	labels map[string]string
	err    error
}{
	{map[string]string{"hello": "world"}, nil},
	{map[string]string{"hello": "world="}, nil},
	{map[string]string{"hello": "wor=ld"}, nil},
	{map[string]string{"hello": "=world"}, nil},
	{map[string]string{"hello=": "world"}, ErrLabelInvalidKey},
	{map[string]string{"hel=lo": "world"}, ErrLabelInvalidKey},
	{map[string]string{"=hello": "world"}, ErrLabelInvalidKey},

	{map[string]string{"foo": "bar", "hello=": "world"}, ErrLabelInvalidKey},
	{map[string]string{"foo": "bar", "hel=lo": "world"}, ErrLabelInvalidKey},
	{map[string]string{"foo": "bar", "=hello": "world"}, ErrLabelInvalidKey},
}

func TestValidateLabels(t *testing.T) {
	for _, test := range validateLabelsTests {
		assert.Equal(t, test.err, validateLabels(test.labels), "labels: %v expected: %+v", test.labels, test.err)
	}
}
