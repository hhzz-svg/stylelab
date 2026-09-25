package insight

import "math"

// Tidy tree layout: Buchheim, Jünger & Leipert, "Improving Walker's
// Algorithm to Run in Linear Time" (2002), itself the Reingold-Tilford
// algorithm extended to trees of any degree. The guarantees the tests
// check: siblings keep their order, nodes on one level are at least one
// unit apart, a parent is centred over its children, and smaller subtrees
// sitting between large ones are spread evenly instead of piling left.

const siblingGap = 1.0

type walkNode struct {
	node     *LineageNode
	parent   *walkNode
	children []*walkNode
	number   int // 1-based index among siblings

	prelim, mod, change, shift float64
	thread, ancestor           *walkNode
}

func newWalk(n *LineageNode, parent *walkNode, number int) *walkNode {
	w := &walkNode{node: n, parent: parent, number: number}
	w.ancestor = w
	for i, c := range n.Children {
		w.children = append(w.children, newWalk(c, w, i+1))
	}
	return w
}

func (v *walkNode) leftSibling() *walkNode {
	if v.parent == nil || v.number == 1 {
		return nil
	}
	return v.parent.children[v.number-2]
}

func (v *walkNode) leftmostSibling() *walkNode {
	if v.parent == nil || v.number == 1 {
		return nil
	}
	return v.parent.children[0]
}

func (v *walkNode) nextLeft() *walkNode {
	if len(v.children) > 0 {
		return v.children[0]
	}
	return v.thread
}

func (v *walkNode) nextRight() *walkNode {
	if len(v.children) > 0 {
		return v.children[len(v.children)-1]
	}
	return v.thread
}

func firstWalk(v *walkNode) {
	if len(v.children) == 0 {
		if w := v.leftSibling(); w != nil {
			v.prelim = w.prelim + siblingGap
		}
		return
	}
	defaultAncestor := v.children[0]
	for _, w := range v.children {
		firstWalk(w)
		defaultAncestor = apportion(w, defaultAncestor)
	}
	executeShifts(v)
	mid := (v.children[0].prelim + v.children[len(v.children)-1].prelim) / 2
	if w := v.leftSibling(); w != nil {
		v.prelim = w.prelim + siblingGap
		v.mod = v.prelim - mid
	} else {
		v.prelim = mid
	}
}

// apportion pushes v's subtree right until its left contour clears the
// right contour of the siblings before it, spreading the shift over the
// subtrees in between.
func apportion(v, defaultAncestor *walkNode) *walkNode {
	w := v.leftSibling()
	if w == nil {
		return defaultAncestor
	}
	vip, vop := v, v
	vim, vom := w, v.leftmostSibling()
	sip, sop, sim, som := vip.mod, vop.mod, vim.mod, vom.mod
	for vim.nextRight() != nil && vip.nextLeft() != nil {
		vim, vip = vim.nextRight(), vip.nextLeft()
		vom, vop = vom.nextLeft(), vop.nextRight()
		vop.ancestor = v
		shift := (vim.prelim + sim) - (vip.prelim + sip) + siblingGap
		if shift > 0 {
			moveSubtree(ancestorOf(vim, v, defaultAncestor), v, shift)
			sip += shift
			sop += shift
		}
		sim += vim.mod
		sip += vip.mod
		som += vom.mod
		sop += vop.mod
	}
	if vim.nextRight() != nil && vop.nextRight() == nil {
		vop.thread = vim.nextRight()
		vop.mod += sim - sop
	}
	if vip.nextLeft() != nil && vom.nextLeft() == nil {
		vom.thread = vip.nextLeft()
		vom.mod += sip - som
		defaultAncestor = v
	}
	return defaultAncestor
}

func moveSubtree(wm, wp *walkNode, shift float64) {
	subtrees := float64(wp.number - wm.number)
	wp.change -= shift / subtrees
	wp.shift += shift
	wm.change += shift / subtrees
	wp.prelim += shift
	wp.mod += shift
}

func executeShifts(v *walkNode) {
	shift, change := 0.0, 0.0
	for i := len(v.children) - 1; i >= 0; i-- {
		w := v.children[i]
		w.prelim += shift
		w.mod += shift
		change += w.change
		shift += w.shift + change
	}
}

func ancestorOf(vim, v, defaultAncestor *walkNode) *walkNode {
	if vim.ancestor.parent == v.parent {
		return vim.ancestor
	}
	return defaultAncestor
}

func secondWalk(v *walkNode, m float64, depth int, minX *float64) {
	v.node.X = v.prelim + m
	v.node.Depth = depth
	*minX = math.Min(*minX, v.node.X)
	for _, w := range v.children {
		secondWalk(w, m+v.mod, depth+1, minX)
	}
}

func shiftTree(n *LineageNode, dx float64, maxX *float64, maxDepth *int) {
	n.X += dx
	*maxX = math.Max(*maxX, n.X)
	if n.Depth > *maxDepth {
		*maxDepth = n.Depth
	}
	for _, c := range n.Children {
		shiftTree(c, dx, maxX, maxDepth)
	}
}

// layoutTree lays out one tree with its leftmost node at x = 0.
func layoutTree(root *LineageNode) {
	w := newWalk(root, nil, 1)
	firstWalk(w)
	minX := math.Inf(1)
	secondWalk(w, -w.prelim, 0, &minX)
	var maxX float64
	var maxDepth int
	shiftTree(root, -minX, &maxX, &maxDepth)
}

// layoutForest lays the trees side by side, each starting one gap after
// the previous one's rightmost node. Returns the forest's width and depth.
func layoutForest(roots []*LineageNode) (float64, int) {
	offset, width, depth := 0.0, 0.0, 0
	for _, r := range roots {
		layoutTree(r)
		var maxX float64
		var maxDepth int
		shiftTree(r, offset, &maxX, &maxDepth)
		width = math.Max(width, maxX)
		if maxDepth > depth {
			depth = maxDepth
		}
		offset = maxX + siblingGap
	}
	return width, depth
}
