package swim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNodeLabels(t *testing.T) {
	testNode := newChannelNode(t)
	defer testNode.Destroy()

	labels := testNode.node.Labels()
	require.NotNil(t, labels)

	err := labels.Set("hello", "world")
	assert.NoError(t, err, "expected no error when setting labels")

	value, has := labels.Get("hello")
	assert.True(t, has, "expected hello label to be set")
	assert.Equal(t, "world", value, "expected 'hello' label to be set to the value 'world'")

	m := labels.AsMap()
	assert.Equal(t, map[string]string{"hello": "world"}, m)

	removed, err := labels.Remove("hello")
	assert.NoError(t, err, "expected no error while removing the label with key 'hello'")
	assert.True(t, removed, "expected 'hello' label to be removed")

	value, has = labels.Get("hello")
	assert.False(t, has, "expected hello label to not be set after remove")
	assert.NotEqual(t, "world", value, "expected 'hello' label to not be set to the value 'world' after remove")
}
