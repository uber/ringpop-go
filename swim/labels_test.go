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
	{"__hello__", true},
	{"_hello", false},
	{"h__ello", false},
	{"hello__", false},
	{"_h_ello", false},
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

func TestNodeLabelsInternal(t *testing.T) {
	testNode := newChannelNode(t)
	defer testNode.Destroy()

	labels := testNode.node.Labels()
	require.NotNil(t, labels)

	err := labels.Set("__internal", "protected value")
	assert.Error(t, err, "expected error while setting an internal key via the public interface")

	value, has := labels.Get("__internal")
	assert.False(t, has, "expected the internal label to not be set")
	assert.NotEqual(t, "protected value", value, "expected the 'protected value' to not be returned after setting it gave an error")

	removed, err := labels.Remove("__internal")
	assert.Error(t, err, "expected error while removing an internal key via the public interface")
	assert.False(t, removed, "expected that not all labels have been removed")

}
