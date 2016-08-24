package swim

import (
	"errors"
	"strings"

	"github.com/uber/ringpop-go/util"
)

var (
	// DefaultLabelOptions contain the default values to be used to limit the
	// amount of data being gossipped over the network. The defaults have been
	// chosen with the following assumptions.
	// 1. 1000 node cluster
	// 2. Every byte available is used (yes, characters are not bytes but these
	//    are ballpark figures to protect developers)
	// 3. Worst case would have continues fullsync's, meaning the complete
	//    memberlist being sent over the wire 5 times a second.
	// When all contiditions are met the Labels would add the following load
	// (non-compressed) to the network:
	//    (32+128)*5*1000*5*8 = ~32mbit/s
	DefaultLabelOptions = LabelOptions{
		KeySize:   32,
		ValueSize: 128,
		Count:     5,
	}

	// ErrLabelSizeExceeded indicates that an operation on labels would exceed
	// the configured size limits on labels. This is to prevent applications
	// bloating the gossip protocol with too much data. Ringpop can be
	// configured with the settings to control the amount of data that can be
	// put in a nodes labels.
	ErrLabelSizeExceeded = errors.New("label operation exceeds configured label limits")

	// ErrLabelInternalKey is an error that is returned when an application
	// tries to set a label in the internal namespace that is used by ringpop
	ErrLabelInternalKey = errors.New("label can't be altered by application because it is in the internal ringpop namespace")

	labelsInternalNamespacePrefix = "__"
)

// LabelOptions controlls the limits on labels. Since labels are gossiped on
// every ping/ping-req/fullsync we need to limit the amount of data an
// application stores in their labels. When needed the defaults can be
// overwritten during the construction of ringpop. This should be done with care
// to not overwhelm the network with data.
type LabelOptions struct {
	// KeySize is the length a key may use at most
	KeySize int

	// ValueSize is the length a value may use at most
	ValueSize int

	// Count is the number of maximum allowed (public) labels on a node
	Count int
}

func mergeLabelOptions(opts LabelOptions, def LabelOptions) LabelOptions {
	return LabelOptions{
		KeySize:   util.SelectInt(opts.KeySize, def.KeySize),
		ValueSize: util.SelectInt(opts.ValueSize, def.ValueSize),
		Count:     util.SelectInt(opts.Count, def.Count),
	}
}

func (lo LabelOptions) validateLabel(current map[string]string, key, value string) error {
	if isInternalLabel(key) {
		// we ignore internal labels in the limits
		return nil
	}

	if len(key) > lo.KeySize || len(value) > lo.ValueSize {
		// if the either the key or the value of the label is bigger then
		// the max allowed size an error is returned
		return ErrLabelSizeExceeded
	}

	// keep track of all labels that are new in this operation to calculate if
	// we are exceeding the maximum allowed number of labels
	additionalLabelCount := 0

	_, has := current[key]
	if !has {
		// only add a count to the countAfter if the key we are looking
		// at is a new key
		additionalLabelCount++
	}

	// get the count of the current labels, internal labels will not be counted
	currentCount := countNonInternalLabels(current)

	if additionalLabelCount > 0 && (currentCount+additionalLabelCount) > lo.Count {
		// Only when we add additional labels we check if the new count
		// (exluding internal labels) will exceed the amount of labels that is
		// configured. If that is the case we will return an error
		return ErrLabelSizeExceeded
	}

	// all is ok
	return nil
}

func (lo LabelOptions) validateLabels(current map[string]string, additional map[string]string) error {
	// keep track of all labels that are new in this operation to calculate if
	// we are exceeding the maximum allowed number of labels
	additionalLabelCount := 0

	for key, value := range additional {
		if isInternalLabel(key) {
			// we ignore internal labels in the limits
			continue
		}

		if len(key) > lo.KeySize || len(value) > lo.ValueSize {
			// if the either the key or the value of the label is bigger then
			// the max allowed size an error is returned
			return ErrLabelSizeExceeded
		}

		_, has := current[key]
		if !has {
			// only add a count to the countAfter if the key we are looking
			// at is a new key
			additionalLabelCount++
		}
	}

	// get the count of the current labels, internal labels will not be counted
	currentCount := countNonInternalLabels(current)

	if additionalLabelCount > 0 && (currentCount+additionalLabelCount) > lo.Count {
		// Only when we add additional labels we check if the new count
		// (exluding internal labels) will exceed the amount of labels that is
		// configured. If that is the case we will return an error
		return ErrLabelSizeExceeded
	}

	// all is ok
	return nil
}

func isInternalLabel(key string) bool {
	return strings.HasPrefix(key, labelsInternalNamespacePrefix)
}

func countNonInternalLabels(labels map[string]string) int {
	count := 0
	for key := range labels {
		if isInternalLabel(key) {
			continue
		}
		count++
	}
	return count
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
	if isInternalLabel(key) {
		return ErrLabelInternalKey
	}
	return n.node.memberlist.SetLocalLabel(key, value)
}

// Remove a key from the labels
func (n *NodeLabels) Remove(key string) (removed bool, err error) {
	if isInternalLabel(key) {
		return false, ErrLabelInternalKey
	}
	return n.node.memberlist.RemoveLocalLabels(key), nil
}

// AsMap gets a readonly copy of all the labels assigned to Node. Changes to the
// map will not be refelected in the node.
func (n *NodeLabels) AsMap() map[string]string {
	return n.node.memberlist.LocalLabelsAsMap()
}
