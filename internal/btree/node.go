package btree

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/leandroasilva/gothinkdb/internal/storage"
)

// B-Tree constants
const (
	// MaxKeysPerNode is the maximum number of keys in a node
	// For a 4KB block, we can fit approximately:
	// - Leaf node: ~50 keys (80 bytes per key-value pair)
	// - Internal node: ~250 keys (16 bytes per key-pointer pair)
	MaxLeafKeys     = 50
	MaxInternalKeys = 250

	// MinKeysPerNode is the minimum number of keys (for balance)
	MinLeafKeys     = MaxLeafKeys / 2
	MinInternalKeys = MaxInternalKeys / 2
)

// NodeType represents the type of a B-Tree node
type NodeType uint8

const (
	NodeTypeLeaf     NodeType = 1
	NodeTypeInternal NodeType = 2
)

// Node represents a B-Tree node stored in a block
type Node struct {
	blockID  storage.BlockID
	nodeType NodeType
	numKeys  uint16
}

// LeafNode represents a leaf node in the B-Tree
type LeafNode struct {
	Node
	keys   [][]byte        // Sorted keys
	values [][]byte        // Corresponding values
	right  storage.BlockID // Right sibling (for range scans)
}

// InternalNode represents an internal node in the B-Tree
type InternalNode struct {
	Node
	keys     [][]byte          // Sorted keys (separators)
	children []storage.BlockID // Child pointers (len = len(keys) + 1)
}

// NewLeafNode creates a new leaf node
func NewLeafNode(blockID storage.BlockID) *LeafNode {
	return &LeafNode{
		Node: Node{
			blockID:  blockID,
			nodeType: NodeTypeLeaf,
			numKeys:  0,
		},
		keys:   make([][]byte, 0, MaxLeafKeys),
		values: make([][]byte, 0, MaxLeafKeys),
		right:  storage.InvalidBlockID,
	}
}

// NewInternalNode creates a new internal node
func NewInternalNode(blockID storage.BlockID) *InternalNode {
	return &InternalNode{
		Node: Node{
			blockID:  blockID,
			nodeType: NodeTypeInternal,
			numKeys:  0,
		},
		keys:     make([][]byte, 0, MaxInternalKeys),
		children: make([]storage.BlockID, 0, MaxInternalKeys+1),
	}
}

// IsFull returns true if the node is full
func (n *LeafNode) IsFull() bool {
	return len(n.keys) >= MaxLeafKeys
}

// IsFull returns true if the node is full
func (n *InternalNode) IsFull() bool {
	return len(n.children) >= MaxInternalKeys+1
}

// IsUnderflow returns true if the node has too few keys
func (n *LeafNode) IsUnderflow() bool {
	return len(n.keys) < MinLeafKeys
}

// IsUnderflow returns true if the node has too few keys
func (n *InternalNode) IsUnderflow() bool {
	return len(n.children) < MinInternalKeys+1
}

// Serialize serializes the leaf node to bytes
func (n *LeafNode) Serialize() []byte {
	buf := new(bytes.Buffer)

	// Write header
	buf.WriteByte(byte(n.nodeType))
	binary.Write(buf, binary.LittleEndian, n.numKeys)
	binary.Write(buf, binary.LittleEndian, n.right)

	// Write keys and values
	for i := 0; i < int(n.numKeys); i++ {
		// Write key length and key
		binary.Write(buf, binary.LittleEndian, uint16(len(n.keys[i])))
		buf.Write(n.keys[i])

		// Write value length and value
		binary.Write(buf, binary.LittleEndian, uint16(len(n.values[i])))
		buf.Write(n.values[i])
	}

	return buf.Bytes()
}

// Deserialize deserializes a leaf node from bytes
func DeserializeLeaf(blockID storage.BlockID, data []byte) (*LeafNode, error) {
	if len(data) < 7 { // Minimum header size
		return nil, fmt.Errorf("data too small")
	}

	buf := bytes.NewReader(data)
	node := NewLeafNode(blockID)

	// Read header
	var nodeType uint8
	binary.Read(buf, binary.LittleEndian, &nodeType)
	if NodeType(nodeType) != NodeTypeLeaf {
		return nil, fmt.Errorf("not a leaf node")
	}

	binary.Read(buf, binary.LittleEndian, &node.numKeys)
	binary.Read(buf, binary.LittleEndian, &node.right)

	// Read keys and values
	for i := 0; i < int(node.numKeys); i++ {
		// Read key
		var keyLen uint16
		binary.Read(buf, binary.LittleEndian, &keyLen)
		key := make([]byte, keyLen)
		buf.Read(key)
		node.keys = append(node.keys, key)

		// Read value
		var valLen uint16
		binary.Read(buf, binary.LittleEndian, &valLen)
		value := make([]byte, valLen)
		buf.Read(value)
		node.values = append(node.values, value)
	}

	return node, nil
}

// Serialize serializes the internal node to bytes
func (n *InternalNode) Serialize() []byte {
	buf := new(bytes.Buffer)

	// Write header
	buf.WriteByte(byte(n.nodeType))
	binary.Write(buf, binary.LittleEndian, n.numKeys)

	// Write number of children
	numChildren := uint16(len(n.children))
	binary.Write(buf, binary.LittleEndian, numChildren)

	// Write children
	for _, child := range n.children {
		binary.Write(buf, binary.LittleEndian, child)
	}

	// Write keys
	for _, key := range n.keys {
		binary.Write(buf, binary.LittleEndian, uint16(len(key)))
		buf.Write(key)
	}

	return buf.Bytes()
}

// Deserialize deserializes an internal node from bytes
func DeserializeInternal(blockID storage.BlockID, data []byte) (*InternalNode, error) {
	if len(data) < 7 { // Minimum header size
		return nil, fmt.Errorf("data too small")
	}

	buf := bytes.NewReader(data)
	node := NewInternalNode(blockID)

	// Read header
	var nodeType uint8
	binary.Read(buf, binary.LittleEndian, &nodeType)
	if NodeType(nodeType) != NodeTypeInternal {
		return nil, fmt.Errorf("not an internal node")
	}

	binary.Read(buf, binary.LittleEndian, &node.numKeys)

	// Read children
	var numChildren uint16
	binary.Read(buf, binary.LittleEndian, &numChildren)
	node.children = make([]storage.BlockID, numChildren)
	for i := 0; i < int(numChildren); i++ {
		binary.Read(buf, binary.LittleEndian, &node.children[i])
	}

	// Read keys
	for i := 0; i < int(node.numKeys); i++ {
		var keyLen uint16
		binary.Read(buf, binary.LittleEndian, &keyLen)
		key := make([]byte, keyLen)
		buf.Read(key)
		node.keys = append(node.keys, key)
	}

	return node, nil
}

// Insert inserts a key-value pair into the leaf node
func (n *LeafNode) Insert(key, value []byte) error {
	if n.IsFull() {
		return fmt.Errorf("leaf node is full")
	}

	// Find insertion position
	pos := n.findKey(key)

	// Check if key already exists
	if pos < len(n.keys) && bytes.Equal(n.keys[pos], key) {
		// Update existing value
		n.values[pos] = value
		return nil
	}

	// Insert new key-value pair
	n.keys = append(n.keys, nil)
	n.values = append(n.values, nil)
	copy(n.keys[pos+1:], n.keys[pos:])
	copy(n.values[pos+1:], n.values[pos:])
	n.keys[pos] = key
	n.values[pos] = value
	n.numKeys++

	return nil
}

// Delete deletes a key from the leaf node
func (n *LeafNode) Delete(key []byte) bool {
	pos := n.findKey(key)
	if pos >= len(n.keys) || !bytes.Equal(n.keys[pos], key) {
		return false
	}

	// Remove key and value
	n.keys = append(n.keys[:pos], n.keys[pos+1:]...)
	n.values = append(n.values[:pos], n.values[pos+1:]...)
	n.numKeys--

	return true
}

// Get retrieves a value by key
func (n *LeafNode) Get(key []byte) ([]byte, bool) {
	pos := n.findKey(key)
	if pos >= len(n.keys) || !bytes.Equal(n.keys[pos], key) {
		return nil, false
	}
	return n.values[pos], true
}

// findKey finds the position where key should be inserted
func (n *LeafNode) findKey(key []byte) int {
	low, high := 0, len(n.keys)
	for low < high {
		mid := (low + high) / 2
		if bytes.Compare(n.keys[mid], key) < 0 {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low
}

// Split splits a full leaf node and returns the new right node
func (n *LeafNode) Split(newBlockID storage.BlockID) *LeafNode {
	mid := len(n.keys) / 2

	// Create new right node
	right := NewLeafNode(newBlockID)
	right.right = n.right
	n.right = newBlockID

	// Move half the keys to the right node
	right.keys = append(right.keys, n.keys[mid:]...)
	right.values = append(right.values, n.values[mid:]...)
	right.numKeys = uint16(len(right.keys))

	// Truncate left node
	n.keys = n.keys[:mid]
	n.values = n.values[:mid]
	n.numKeys = uint16(len(n.keys))

	return right
}

// FindChild finds the child index for a given key
func (n *InternalNode) FindChild(key []byte) int {
	low, high := 0, len(n.keys)
	for low < high {
		mid := (low + high) / 2
		if bytes.Compare(n.keys[mid], key) <= 0 {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low
}

// Insert inserts a key and right child into the internal node
func (n *InternalNode) Insert(key []byte, rightChild storage.BlockID) error {
	if n.IsFull() {
		return fmt.Errorf("internal node is full")
	}

	pos := n.FindChild(key)

	// Insert key
	n.keys = append(n.keys, nil)
	copy(n.keys[pos+1:], n.keys[pos:])
	n.keys[pos] = key

	// Insert child
	n.children = append(n.children, storage.InvalidBlockID)
	copy(n.children[pos+2:], n.children[pos+1:])
	n.children[pos+1] = rightChild

	n.numKeys++
	return nil
}

// Split splits a full internal node and returns the new right node
func (n *InternalNode) Split(newBlockID storage.BlockID) (*InternalNode, []byte) {
	mid := len(n.children) / 2

	// Create new right node
	right := NewInternalNode(newBlockID)

	// Move half the children to the right node
	right.children = append(right.children, n.children[mid:]...)

	// The middle key is promoted to the parent
	promotedKey := n.keys[mid-1]

	// Move keys to the right node (excluding the promoted key)
	right.keys = append(right.keys, n.keys[mid:]...)
	right.numKeys = uint16(len(right.keys))

	// Truncate left node
	n.keys = n.keys[:mid-1]
	n.children = n.children[:mid]
	n.numKeys = uint16(len(n.keys))

	return right, promotedKey
}
