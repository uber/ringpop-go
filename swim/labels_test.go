package swim

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixtureExceedKeySize         = strings.Repeat("a", 33)
	fixtureExceedKeySizeInternal = labelsInternalNamespacePrefix + strings.Repeat("a", 33)
	fixtureExceedValueSize       = strings.Repeat("b", 129)

	fixtureLabelsOneNew = map[string]string{
		"newkey": "some value",
	}

	fixtureLabelsOneOverwrite = map[string]string{
		"key1": "new value for 1",
	}

	fixtureLabelsInternalNew = map[string]string{
		"__internal2": "internal value2",
	}

	fixtureLabelsCombinationNew = map[string]string{
		"newkey":      "some value",
		"__internal2": "internal value2",
	}

	fixtureLabelsCombinationOverwrite = map[string]string{
		"key1":        "new value for 1",
		"__internal1": "internal value1",
	}

	fixtureLabelsRoom = map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}

	fixtureLabelsRoomWithInternal = map[string]string{
		"key1":        "value1",
		"key2":        "value2",
		"key3":        "value3",
		"key4":        "value4",
		"__internal1": "internal value1",
	}

	fixtureExactLabels = map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
	}

	fixtureExactLabelsWithInternal = map[string]string{
		"key1":        "value1",
		"key2":        "value2",
		"key3":        "value3",
		"key4":        "value4",
		"key5":        "value5",
		"__internal1": "internal value1",
	}

	fixtureTooBigLabels = map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
		"key6": "value6",
	}

	fixtureTooBigLabelsWithInternal = map[string]string{
		"key1":        "value1",
		"key2":        "value2",
		"key3":        "value3",
		"key4":        "value4",
		"key5":        "value5",
		"key6":        "value6",
		"__internal1": "internal value1",
	}

	fixtureLabelOptions = LabelOptions{
		KeySize:   32,
		ValueSize: 128,
		Count:     5,
	}
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

func TestMergeLabelOptions(t *testing.T) {
	empty := LabelOptions{}
	merged := mergeLabelOptions(empty, fixtureLabelOptions)
	assert.Equal(t, fixtureLabelOptions, merged, "expected an empty LabelOptions struct to equal the fixtureLabelOptions when merged")

	onlyKeySize := LabelOptions{KeySize: 1}
	merged = mergeLabelOptions(onlyKeySize, fixtureLabelOptions)
	assert.Equal(t, 1, merged.KeySize, "expected the KeySize to be set to the value set in the optional LabelOptions")
	assert.Equal(t, fixtureLabelOptions.ValueSize, merged.ValueSize, "expected the ValueSize to be taken from the default options")
	assert.Equal(t, fixtureLabelOptions.Count, merged.Count, "expected the Count to be taken from the default options")

	onlyValueSize := LabelOptions{ValueSize: 1}
	merged = mergeLabelOptions(onlyValueSize, fixtureLabelOptions)
	assert.Equal(t, 1, merged.ValueSize, "expected the ValueSize to be set to the value set in the optional LabelOptions")
	assert.Equal(t, fixtureLabelOptions.KeySize, merged.KeySize, "expected the KeySize to be taken from the default options")
	assert.Equal(t, fixtureLabelOptions.Count, merged.Count, "expected the Count to be taken from the default options")

	onlyCount := LabelOptions{Count: 1}
	merged = mergeLabelOptions(onlyCount, fixtureLabelOptions)
	assert.Equal(t, 1, merged.Count, "expected the Count to be set to the value set in the optional LabelOptions")
	assert.Equal(t, fixtureLabelOptions.ValueSize, merged.ValueSize, "expected the ValueSize to be taken from the default options")
	assert.Equal(t, fixtureLabelOptions.KeySize, merged.KeySize, "expected the KeySize to be taken from the default options")
}

func TestLabelOptions_validateLabel(t *testing.T) {
	var err error

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, "newkey", "some value")
	assert.NoError(t, err, "expected to be able to add a label when there is room in the map")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoomWithInternal, "newkey", "some value")
	assert.NoError(t, err, "expected to be able to add a label when there is room in the map (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, "key1", "new value for 1")
	assert.NoError(t, err, "expected to be able to overwrite a label when there is room")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoomWithInternal, "key1", "new value for 1")
	assert.NoError(t, err, "expected to be able to overwrite a label when there is room (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureExactLabels, "newkey", "some value")
	assert.Error(t, err, "expected an error when adding a new key to a map that is at capacity")

	err = fixtureLabelOptions.validateLabel(fixtureExactLabelsWithInternal, "newkey", "some value")
	assert.Error(t, err, "expected an error when adding a new key to a map that is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureExactLabels, "key1", "new value for 1")
	assert.NoError(t, err, "expected to be able to overwrite a label even though the map is at capacity")

	err = fixtureLabelOptions.validateLabel(fixtureExactLabelsWithInternal, "key1", "new value for 1")
	assert.NoError(t, err, "expected to be able to overwrite a label even though the map is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureTooBigLabels, "newkey", "some value")
	assert.Error(t, err, "expected an error when adding a new key to a map that is over capacity")

	err = fixtureLabelOptions.validateLabel(fixtureTooBigLabelsWithInternal, "newkey", "some value")
	assert.Error(t, err, "expected an error when adding a new key to a map that is over capacity (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureTooBigLabels, "key1", "new value for 1")
	assert.NoError(t, err, "expected no error when overwriting a key on a map that is over capacity")

	err = fixtureLabelOptions.validateLabel(fixtureTooBigLabelsWithInternal, "key1", "new value for 1")
	assert.NoError(t, err, "expected no error when overwriting a key on a map that is over capacity (with internal)")

	// make sure that internal labels are still accepted to allow for ringpop internals to proceed
	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, "__internal2", "internal value2")
	assert.NoError(t, err, "expected no error when adding a internal label when there is room in the map")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoomWithInternal, "__internal2", "internal value2")
	assert.NoError(t, err, "expected no error when adding a internal label when there is room in the map (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureExactLabels, "__internal2", "internal value2")
	assert.NoError(t, err, "expected no error when adding a internal label to a map that is at capacity")

	err = fixtureLabelOptions.validateLabel(fixtureExactLabelsWithInternal, "__internal2", "internal value2")
	assert.NoError(t, err, "expected no error when adding a internal label to a map that is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabel(fixtureTooBigLabels, "__internal2", "internal value2")
	assert.NoError(t, err, "expected no error when adding a internal label when the map is over capacity")

	err = fixtureLabelOptions.validateLabel(fixtureTooBigLabelsWithInternal, "__internal2", "internal value2")
	assert.NoError(t, err, "expected no error when adding a internal label when the map is over capacity (with internal)")

	// tests for key and value size exceeding
	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, fixtureExceedKeySize, "some value")
	assert.Error(t, err, "expected error when the key length exceeds the configured length")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, "newkey", fixtureExceedValueSize)
	assert.Error(t, err, "expected error when the value length exceeds the configured length")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, fixtureExceedKeySizeInternal, "some value")
	assert.NoError(t, err, "expected no error when the key length exceeds the configured length for a internal key")

	err = fixtureLabelOptions.validateLabel(fixtureLabelsRoom, fixtureExceedKeySizeInternal, fixtureExceedValueSize)
	assert.NoError(t, err, "expected no error when the value length exceeds the configured length for a internal key")
}

func TestLabelOptions_validateLabels(t *testing.T) {
	var err error

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, fixtureLabelsOneNew)
	assert.NoError(t, err, "expected to be able to add a label when there is room in the map")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoomWithInternal, fixtureLabelsOneNew)
	assert.NoError(t, err, "expected to be able to add a label when there is room in the map (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, fixtureLabelsOneOverwrite)
	assert.NoError(t, err, "expected to be able to overwrite a label when there is room")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoomWithInternal, fixtureLabelsOneOverwrite)
	assert.NoError(t, err, "expected to be able to overwrite a label when there is room (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabels, fixtureLabelsOneNew)
	assert.Error(t, err, "expected an error when adding a new key to a map that is at capacity")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabelsWithInternal, fixtureLabelsOneNew)
	assert.Error(t, err, "expected an error when adding a new key to a map that is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabels, fixtureLabelsOneOverwrite)
	assert.NoError(t, err, "expected to be able to overwrite a label even though the map is at capacity")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabelsWithInternal, fixtureLabelsOneOverwrite)
	assert.NoError(t, err, "expected to be able to overwrite a label even though the map is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabels, fixtureLabelsOneNew)
	assert.Error(t, err, "expected an error when adding a new key to a map that is over capacity")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabelsWithInternal, fixtureLabelsOneNew)
	assert.Error(t, err, "expected an error when adding a new key to a map that is over capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabels, fixtureLabelsOneOverwrite)
	assert.NoError(t, err, "expected no error when overwriting a key on a map that is over capacity")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabelsWithInternal, fixtureLabelsOneOverwrite)
	assert.NoError(t, err, "expected no error when overwriting a key on a map that is over capacity (with internal)")

	// make sure that internal labels are still accepted to allow for ringpop internals to proceed
	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, fixtureLabelsInternalNew)
	assert.NoError(t, err, "expected no error when adding a internal label when there is room in the map")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoomWithInternal, fixtureLabelsInternalNew)
	assert.NoError(t, err, "expected no error when adding a internal label when there is room in the map (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabels, fixtureLabelsInternalNew)
	assert.NoError(t, err, "expected no error when adding a internal label to a map that is at capacity")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabelsWithInternal, fixtureLabelsInternalNew)
	assert.NoError(t, err, "expected no error when adding a internal label to a map that is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabels, fixtureLabelsInternalNew)
	assert.NoError(t, err, "expected no error when adding a internal label when the map is over capacity")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabelsWithInternal, fixtureLabelsInternalNew)
	assert.NoError(t, err, "expected no error when adding a internal label when the map is over capacity (with internal)")

	// tests for key and value size exceeding
	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, map[string]string{fixtureExceedKeySize: "some value"})
	assert.Error(t, err, "expected error when the key length exceeds the configured length")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, map[string]string{"newkey": fixtureExceedValueSize})
	assert.Error(t, err, "expected error when the value length exceeds the configured length")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, map[string]string{fixtureExceedKeySizeInternal: "some value"})
	assert.NoError(t, err, "expected no error when the key length exceeds the configured length for a internal key")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, map[string]string{fixtureExceedKeySizeInternal: fixtureExceedValueSize})
	assert.NoError(t, err, "expected no error when the value length exceeds the configured length for a internal key")

	// tests for a combination of both internal an non internal labels
	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, fixtureLabelsCombinationNew)
	assert.NoError(t, err, "expected to be able to add a combination of new internal and non internal labels in a map that has room")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoomWithInternal, fixtureLabelsCombinationNew)
	assert.NoError(t, err, "expected to be able to add a combination of new internal and non internal labels in a map that has room (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabels, fixtureLabelsCombinationNew)
	assert.Error(t, err, "expected error when adding a combination of new internal and non internal labels in a map that is at capacity")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabelsWithInternal, fixtureLabelsCombinationNew)
	assert.Error(t, err, "expected error when adding a combination of new internal and non internal labels in a map that is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabels, fixtureLabelsCombinationNew)
	assert.Error(t, err, "expected error when adding a combination of new internal and non internal labels in a map that is at over capacity")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabelsWithInternal, fixtureLabelsCombinationNew)
	assert.Error(t, err, "expected error when adding a combination of new internal and non internal labels in a map that is at over capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoom, fixtureLabelsCombinationOverwrite)
	assert.NoError(t, err, "expected to be able to add a combination of existing internal and non internal labels in a map that has room")

	err = fixtureLabelOptions.validateLabels(fixtureLabelsRoomWithInternal, fixtureLabelsCombinationOverwrite)
	assert.NoError(t, err, "expected to be able to add a combination of existing internal and non internal labels in a map that has room (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabels, fixtureLabelsCombinationOverwrite)
	assert.NoError(t, err, "expected to be able to add a combination of existing internal and non internal labels in a map that is at capacity")

	err = fixtureLabelOptions.validateLabels(fixtureExactLabelsWithInternal, fixtureLabelsCombinationOverwrite)
	assert.NoError(t, err, "expected to be able to add a combination of existing internal and non internal labels in a map that is at capacity (with internal)")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabels, fixtureLabelsCombinationOverwrite)
	assert.NoError(t, err, "expected to be able to add a combination of existing internal and non internal labels in a map that is at over capacity")

	err = fixtureLabelOptions.validateLabels(fixtureTooBigLabelsWithInternal, fixtureLabelsCombinationOverwrite)
	assert.NoError(t, err, "expected to be able to add a combination of existing internal and non internal labels in a map that is at over capacity (with internal)")
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
