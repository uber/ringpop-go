package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIndexOf(t *testing.T) {
	nums := []string{"0", "1", "2"}

	assert.Equal(t, 0, indexOf(nums, "0"), "expected 0 to be at index 0")
	assert.Equal(t, 1, indexOf(nums, "1"), "expected 1 to be at index 1")
	assert.Equal(t, 2, indexOf(nums, "2"), "expected 2 to be at index 2")
	assert.Equal(t, -1, indexOf(nums, "3"), "expected 3 to not be in slice")
}

func TestTakeNode(t *testing.T) {
	nodes := []string{"0", "1", "2", "3", "4"}

	node := takeNode(&nodes, 2)
	assert.Equal(t, "2", node, "expected to get 2 from slice of nodes")
	assert.Len(t, nodes, 4, "expected nodes to be mutated")
	assert.Equal(t, -1, indexOf(nodes, "2"), "expected 2 to be removed from slice")

	node = takeNode(&nodes, 0)
	assert.Equal(t, "0", node, "expected to get 0 from slice of nodes")
	assert.Len(t, nodes, 3, "expected nodes to be mutated")
	assert.Equal(t, -1, indexOf(nodes, "0"), "expected 2 to be removed from slice")

	node = takeNode(&nodes, 2)
	assert.Equal(t, "4", node, "expected to get 4 from slice of nodes")
	assert.Len(t, nodes, 2, "expected nodes to be mutated")
	assert.Equal(t, -1, indexOf(nodes, "4"), "expected 4 to be removed from slice")

	node = takeNode(&nodes, -1)
	assert.Len(t, nodes, 1, "expected nodes to be mutated, random node taken")

	node = takeNode(&nodes, 100)
	assert.Equal(t, "", node, "expected to get empty string for bad index")
	assert.Len(t, nodes, 1, "expected nodes to stay the same")

	node = takeNode(&nodes, -1)
	assert.Len(t, nodes, 0, "expecte nodes to be empty")

	node = takeNode(&nodes, 0)
	assert.Equal(t, "", node, "expected to get back empty string from empty slice")
	assert.Len(t, nodes, 0, "expecte nodes to be empty")
}

func TestCaptureHost(t *testing.T) {
	hostport := "127.0.0.1:3001"
	badHostport := "127.0..1:3004"

	assert.Equal(t, "127.0.0.1", captureHost(hostport), "expected hostport to be captured")
	assert.Equal(t, "", captureHost(badHostport), "expected empty string as return value")
}

func TestSelectNumOrDefault(t *testing.T) {
	opt, zopt, def := 1, 0, 2

	assert.Equal(t, 1, selectNumOrDefault(opt, def), "expected to get option")
	assert.Equal(t, 2, selectNumOrDefault(zopt, def), "expected to get default")
}

func TestSelectDurationOrDefault(t *testing.T) {
	opt, zopt, def := time.Duration(1), time.Duration(0), time.Duration(2)

	assert.Equal(t, time.Duration(1), selectDurationOrDefault(opt, def), "expected to get option")
	assert.Equal(t, time.Duration(2), selectDurationOrDefault(zopt, def), "expected to get default")
}
