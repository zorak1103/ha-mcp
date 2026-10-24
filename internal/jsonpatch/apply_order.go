package jsonpatch

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/jsonorder"
)

// ApplyWithOrder applies ops like Apply and additionally keeps o, the key
// order of doc, in step with every operation. The order tree is walked in
// parallel with the document: an existing key keeps its position, a new key
// is appended, array items shift with inserts and removals, and inserted
// values take their op's ValueOrder (copy and move take the source's order).
// An object node without recorded keys is first seeded with its sorted keys,
// which is exactly how it renders without an order.
//
// The operation is atomic: on any error the original doc and the original o
// are returned unchanged. Neither o nor any op's ValueOrder is modified.
func ApplyWithOrder(doc any, o *jsonorder.Order, ops []Operation) (any, *jsonorder.Order, error) {
	if err := Validate(ops); err != nil {
		return doc, o, err
	}

	clone, err := deepClone(doc)
	if err != nil {
		return doc, o, fmt.Errorf("failed to clone document: %w", err)
	}
	order := o.Clone()

	for i, op := range ops {
		clone, order, err = applyOneWithOrder(clone, order, op, i)
		if err != nil {
			return doc, o, err
		}
	}
	return clone, order, nil
}

// applyOneWithOrder dispatches a single operation on both trees.
func applyOneWithOrder(doc any, order *jsonorder.Order, op Operation, idx int) (any, *jsonorder.Order, error) {
	switch op.Op {
	case "add":
		return setWithOrder(doc, order, op.Path, op.ValueOrder.Clone(), true, func(d any) (any, error) {
			return applyAdd(d, op.Path, op.Value, idx)
		})
	case "replace":
		return setWithOrder(doc, order, op.Path, op.ValueOrder.Clone(), false, func(d any) (any, error) {
			return applyReplace(d, op.Path, op.Value, idx)
		})
	case "remove":
		return removeWithOrder(doc, order, op.Path, idx)
	case "move":
		return moveWithOrder(doc, order, op.From, op.Path, idx)
	case "copy":
		sub := orderAt(order, doc, op.From).Clone()
		return setWithOrder(doc, order, op.Path, sub, true, func(d any) (any, error) {
			return applyCopy(d, op.From, op.Path, idx)
		})
	default: // "test"; Validate has rejected every other name
		next, err := applyOne(doc, op, idx)
		return next, order, err
	}
}

// moveWithOrder implements "move" as remove-then-add on both trees, each step
// seeing its own document state so array index shifts are handled. It keeps
// applyMove's error behavior; from == path leaves the order untouched.
func moveWithOrder(doc any, order *jsonorder.Order, from, path string, idx int) (any, *jsonorder.Order, error) {
	if from == path {
		next, err := applyMove(doc, from, path, idx)
		return next, order, err
	}
	if strings.HasPrefix(path+"/", from+"/") {
		return doc, order, opError(idx, fmt.Errorf("move: path %q is a child of from %q (RFC 6902 §4.4)", path, from))
	}
	val, err := Get(doc, from)
	if err != nil {
		return doc, order, opError(idx, fmt.Errorf("from path: %w", err))
	}
	sub := orderAt(order, doc, from).Clone()

	doc, order, err = removeWithOrder(doc, order, from, idx)
	if err != nil {
		return doc, order, err
	}
	// val was detached from the document by the remove, so it is not aliased.
	return setWithOrder(doc, order, path, sub, true, func(d any) (any, error) {
		toSegs, segErr := Segments(path)
		if segErr != nil {
			return d, opError(idx, segErr)
		}
		next, setErr := setAtPath(d, toSegs, val, true, path)
		if setErr != nil {
			return d, opError(idx, setErr)
		}
		return next, nil
	})
}

// parentState is what the order update needs to know about the target's
// parent before the document step runs.
type parentState struct {
	existed bool // object parent: the key was already present
	length  int  // array parent: element count before the op
}

// captureParent records the parent state of segs in doc. Navigation failures
// are ignored: the document step reports them.
func captureParent(doc any, segs []string) parentState {
	last := len(segs) - 1
	parent, err := getAtSegs(doc, segs[:last], "")
	if err != nil {
		return parentState{}
	}
	switch p := parent.(type) {
	case map[string]any:
		_, ok := p[segs[last]]
		return parentState{existed: ok}
	case []any:
		return parentState{length: len(p)}
	default:
		return parentState{}
	}
}

// setWithOrder runs a set-style document step (add/replace/copy/move's add)
// and records sub at path in the order tree.
func setWithOrder(doc any, order *jsonorder.Order, path string, sub *jsonorder.Order, insert bool,
	step func(any) (any, error),
) (any, *jsonorder.Order, error) {
	segs, segErr := Segments(path)
	var pre parentState
	if segErr == nil && len(segs) > 0 {
		pre = captureParent(doc, segs)
	}
	next, err := step(doc)
	if err != nil {
		return doc, order, err
	}
	if len(segs) == 0 {
		return next, sub, nil
	}
	last := len(segs) - 1
	root, node, parent := navigateOrder(order, next, segs[:last])
	switch p := parent.(type) {
	case map[string]any:
		orderSetObject(node, p, segs[last], pre.existed, sub)
	case []any:
		orderSetArray(node, pre.length, segs[last], sub, insert)
	}
	return next, root, nil
}

// removeWithOrder runs the "remove" document step and drops path's entry from
// the order tree.
func removeWithOrder(doc any, order *jsonorder.Order, path string, idx int) (any, *jsonorder.Order, error) {
	segs, segErr := Segments(path)
	var pre parentState
	if segErr == nil && len(segs) > 0 {
		pre = captureParent(doc, segs)
	}
	next, err := applyRemove(doc, path, idx)
	if err != nil {
		return doc, order, err
	}
	last := len(segs) - 1
	root, node, parent := navigateOrder(order, next, segs[:last])
	orderRemove(node, parent, pre.length, segs[last])
	return next, root, nil
}

// navigateOrder walks order along segs in parallel with doc, creating missing
// nodes and padding array items to the document length. It returns the
// (possibly new) root, the node at segs and the document value at segs.
func navigateOrder(order *jsonorder.Order, doc any, segs []string) (root, node *jsonorder.Order, value any) {
	if order == nil {
		order = &jsonorder.Order{}
	}
	node, value = order, doc
	for _, seg := range segs {
		var child *jsonorder.Order
		switch d := value.(type) {
		case map[string]any:
			child, value = node.Fields[seg], d[seg]
			if child == nil {
				child = &jsonorder.Order{}
				setOrderField(node, seg, child)
			}
		case []any:
			i, _ := strconv.Atoi(seg) // the document step already validated the index
			padItems(node, len(d))
			child, value = node.Items[i], d[i]
			if child == nil {
				child = &jsonorder.Order{}
				node.Items[i] = child
			}
		}
		node = child
	}
	return order, node, value
}

// orderAt returns the order node at path (navigated in parallel with doc), or
// nil when none is recorded. It never modifies the tree.
func orderAt(order *jsonorder.Order, doc any, path string) *jsonorder.Order {
	segs, err := Segments(path)
	if err != nil {
		return nil
	}
	for _, seg := range segs {
		switch d := doc.(type) {
		case map[string]any:
			order, doc = order.Field(seg), d[seg]
		case []any:
			i, convErr := strconv.Atoi(seg)
			if convErr != nil || i < 0 || i >= len(d) {
				return nil
			}
			order, doc = order.Item(i), d[i]
		default:
			return nil
		}
	}
	return order
}

// orderSetObject records key in an object node after a set-style op: an
// existing key keeps its position, a new key is appended. A node without
// recorded keys is first seeded with the sorted pre-op keys.
func orderSetObject(node *jsonorder.Order, m map[string]any, key string, existed bool, sub *jsonorder.Order) {
	if len(node.Keys) == 0 {
		node.Keys = sortedMapKeys(m)
		if !existed {
			node.Keys = slices.DeleteFunc(node.Keys, func(k string) bool { return k == key })
		}
	}
	if !existed && !slices.Contains(node.Keys, key) {
		node.Keys = append(node.Keys, key)
	}
	setOrderField(node, key, sub)
}

// orderSetArray records sub at seg in an array node whose document had
// length elements before the op; insert shifts later items, "-" appends.
func orderSetArray(node *jsonorder.Order, length int, seg string, sub *jsonorder.Order, insert bool) {
	padItems(node, length)
	i := length
	if seg != "-" {
		i, _ = strconv.Atoi(seg) // the document step already validated the index
	}
	if insert {
		node.Items = slices.Insert(node.Items, i, sub)
		return
	}
	node.Items[i] = sub
}

// orderRemove drops seg from node after a remove: an object member loses its
// key and sub-order, an array item is deleted (length is the pre-op count).
func orderRemove(node *jsonorder.Order, parent any, length int, seg string) {
	if _, isArray := parent.([]any); isArray {
		padItems(node, length)
		i, _ := strconv.Atoi(seg) // the document step already validated the index
		node.Items = slices.Delete(node.Items, i, i+1)
		return
	}
	node.Keys = slices.DeleteFunc(node.Keys, func(k string) bool { return k == seg })
	setOrderField(node, seg, nil)
}

// setOrderField stores sub as key's sub-order (nil removes it), keeping Fields
// nil when empty.
func setOrderField(node *jsonorder.Order, key string, sub *jsonorder.Order) {
	if sub == nil {
		delete(node.Fields, key)
		if len(node.Fields) == 0 {
			node.Fields = nil
		}
		return
	}
	if node.Fields == nil {
		node.Fields = make(map[string]*jsonorder.Order)
	}
	node.Fields[key] = sub
}

// padItems makes node.Items exactly n long: missing entries are nil
// (scalar/unknown), surplus entries from a stale order are dropped.
func padItems(node *jsonorder.Order, n int) {
	if len(node.Items) >= n {
		node.Items = node.Items[:n]
		return
	}
	node.Items = append(node.Items, make([]*jsonorder.Order, n-len(node.Items))...)
}
