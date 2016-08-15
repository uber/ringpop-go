package swim

import (
	"errors"
	"strings"
)

var (
	// labelsBannedKeyCharacters keeps a list of all banned occurances for label keys
	labelsBannedKeyCharacters = []string{
		// the equal sign is used in label checksum calculation. To prevent
		// unintended or forced collisions on the checksums of labels we do not
		// allow the equal sign to be used in the key portion of a label
		"=",
	}

	// ErrLabelInvalidKey is an error that indicates that a key contains a
	// sequence of characters that are not allowed.
	ErrLabelInvalidKey = errors.New("the label key contains an unallowed sequence of characters")
)

// validateLabel validates if the values of both the key and the value are
// allowed to be gossiped around. Preventing the use of characters that have
// special meaning in swim.
func validateLabel(key, value string) error {
	for _, banned := range labelsBannedKeyCharacters {
		if strings.Contains(key, banned) {
			return ErrLabelInvalidKey
		}
	}
	return nil
}

// validateLabels validates if the values for multiple labels, both the key and
// the value are allowed to be gossiped around. Preventing the use of characters
// that have special meaning in swim.
func validateLabels(labels map[string]string) error {
	for key, value := range labels {
		if err := validateLabel(key, value); err != nil {
			return err
		}
	}
	return nil
}

// NodeLabels implements the ringpop.Labels interface and proxies the calls to
// the swim.Node backing the membership protocol.
type NodeLabels struct {
	node *Node
}

// Get the value of a label for this node
func (n *NodeLabels) Get(key string) (value string, has bool) {
	return n.node.memberlist.GetLocalLabel(key)
}

// Set the key to a specific value. Returning an error when it failed eg. when
// the storage capacity for labels has exceed the maximum ammount. (Currently
// the storage limit is not implemented)
func (n *NodeLabels) Set(key, value string) error {
	return n.node.memberlist.SetLocalLabel(key, value)
}

// Remove a key from the labels
func (n *NodeLabels) Remove(key string) (removed bool) {
	return n.node.memberlist.RemoveLocalLabel(key)
}

// AsMap gets a readonly copy of all the labels assigned to Node. Changes to the
// map will not be refelected in the node.
func (n *NodeLabels) AsMap() map[string]string {
	return n.node.memberlist.LocalLabelsAsMap()
}
