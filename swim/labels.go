package swim

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
