package swim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	removed := labels.Remove("hello")
	assert.True(t, removed, "expected 'hello' label to be removed")

	value, has = labels.Get("hello")
	assert.False(t, has, "expected hello label to not be set after remove")
	assert.NotEqual(t, "world", value, "expected 'hello' label to not be set to the value 'world' after remove")
}
