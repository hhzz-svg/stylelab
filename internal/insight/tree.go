package insight

import (
	"sort"
	"strings"
)

// Relations that put one side above the other. The convention is that the
// edge's source is the superior -- the master, the parent, the lord -- as
// in "林远 → 苏晚: 师徒" meaning 林远 is 苏晚's master. Words naming the
// junior side ("徒弟", "子女", "隶属") read the other way.
var (
	hierarchyReversed = []string{"徒弟", "弟子", "子女", "儿子", "女儿", "臣属", "臣子", "仆从", "下属", "属下", "晚辈", "隶属", "成员"}
	hierarchyForward  = []string{"师徒", "师父", "师傅", "师尊", "父子", "父女", "母子", "母女", "父母", "君臣", "主仆", "上下级", "上级", "掌门", "长辈"}
)

// HierarchyDirection classifies a relation: +1 when the source is the
// superior, -1 when the target is, 0 when the relation is not
// hierarchical (盟友, 宿敌, 同门...).
func HierarchyDirection(relation string) int {
	r := strings.TrimSpace(relation)
	for _, w := range hierarchyReversed {
		if strings.Contains(r, w) {
			return -1
		}
	}
	for _, w := range hierarchyForward {
		if strings.Contains(r, w) {
			return 1
		}
	}
	return 0
}

// TreeEntity is a character or faction as the lineage tree needs it.
type TreeEntity struct {
	ID      string
	Name    string
	Kind    string // "character" or "faction"; other kinds are ignored
	Faction string
}

// TreeLink is a relation; only hierarchical ones shape the tree.
type TreeLink struct {
	Source   string
	Target   string
	Relation string
	Weight   float64
	Order    int // creation order, the last tie-break
}

// LineageNode is one node of the laid-out lineage forest.
type LineageNode struct {
	ID string `json:"id"`
	// Virtual nodes stand for a faction that is named on characters but
	// has no node of its own, and for 未归属, the characters with none.
	Virtual  bool   `json:"virtual"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Faction  string `json:"faction"`
	Relation string `json:"relation"` // how it hangs from its parent; "" for faction membership
	// X is the horizontal position in sibling-gap units, Depth the level.
	X            float64        `json:"x"`
	Depth        int            `json:"depth"`
	Children     []*LineageNode `json:"children"`
	ExtraParents []string       `json:"extra_parents"` // other superiors, not drawn as the tree edge
}

// Lineage is the whole forest.
type Lineage struct {
	Roots []*LineageNode `json:"roots"`
	// Cycles lists the ids in each chain of superiors that looped back on
	// itself (A is B's master, B is A's master); one link of each was
	// dropped to make a tree.
	Cycles [][]string `json:"cycles"`
	Width  float64    `json:"width"` // max X
	Depth  int        `json:"depth"` // max Depth
}

// UnassignedID is the virtual root for characters with no faction.
const UnassignedID = "unassigned"

// VirtualFactionID is the id of the stand-in root for a faction named on
// characters but missing from the graph.
func VirtualFactionID(name string) string { return lineageSpec.virtualPrefix + name }

type candidate struct {
	parent   string
	relation string
	weight   float64
	order    int
}

// Relations that put one place inside another. "青云山 → 东洲: 位于"
// reads as 青云山 lies within 东洲; "包含" and "下辖" read the other way.
var (
	placeInside   = []string{"位于", "坐落", "地处", "属于", "隶属", "境内", "之中", "一部分"}
	placeContains = []string{"包含", "辖", "囊括"}
)

// PlaceDirection classifies a relation between places: +1 when the source
// is the larger place, -1 when the target is, 0 otherwise.
func PlaceDirection(relation string) int {
	r := strings.TrimSpace(relation)
	for _, w := range placeInside {
		if strings.Contains(r, w) {
			return -1
		}
	}
	for _, w := range placeContains {
		if strings.Contains(r, w) {
			return 1
		}
	}
	return 0
}

// forestSpec is what differs between the lineage tree and the place tree;
// everything else -- parent choice, loop breaking, layout -- is shared.
type forestSpec struct {
	kinds         map[string]bool  // entity kinds in the tree
	groupKind     string           // the kind others name in their Faction field
	direction     func(string) int // +1 source is the superior, -1 target, 0 not hierarchical
	virtualPrefix string           // id prefix for a named group with no node
	orphanKind    string           // kind that goes under 未归属 when it has no parent; "" for none
}

var lineageSpec = forestSpec{
	kinds:         map[string]bool{"character": true, "faction": true},
	groupKind:     "faction",
	direction:     HierarchyDirection,
	virtualPrefix: "faction:",
	orphanKind:    "character",
}

var placeSpec = forestSpec{
	kinds:     map[string]bool{"location": true},
	groupKind: "location",
	direction: PlaceDirection,
	// A place's "所属" field often names a sect rather than a region, so it
	// only counts when it names another place; no stand-in roots.
	virtualPrefix: "place:",
}

// BuildLineage turns the relationship graph into a forest: factions at the
// top, their members beneath, and masters above disciples, parents above
// children. Each entity gets one tree parent -- a superior of the same
// faction first, then the strongest tie, then the oldest -- and any others
// are kept as extra_parents. Positions come from Buchheim-Walker.
func BuildLineage(entities []TreeEntity, links []TreeLink) Lineage {
	return buildForest(lineageSpec, entities, links)
}

// BuildPlaces arranges the locations as a geography: regions above the
// places inside them, from 位于 / 包含 relations and from a location's
// "所属" field when it names another location.
func BuildPlaces(entities []TreeEntity, links []TreeLink) Lineage {
	return buildForest(placeSpec, entities, links)
}

func buildForest(spec forestSpec, entities []TreeEntity, links []TreeLink) Lineage {
	byID := map[string]TreeEntity{}
	factionByName := map[string]string{}
	var ids []string
	for _, e := range entities {
		if !spec.kinds[e.Kind] {
			continue
		}
		byID[e.ID] = e
		ids = append(ids, e.ID)
		if e.Kind == spec.groupKind {
			if _, dup := factionByName[e.Name]; !dup {
				factionByName[e.Name] = e.ID
			}
		}
	}
	sort.Strings(ids)

	affiliation := func(e TreeEntity) string {
		if e.Faction != "" && e.Faction != e.Name {
			return e.Faction
		}
		return ""
	}

	// Superiors from hierarchical relations.
	candidates := map[string][]candidate{}
	for _, l := range links {
		dir := spec.direction(l.Relation)
		if dir == 0 {
			continue
		}
		sup, sub := l.Source, l.Target
		if dir < 0 {
			sup, sub = sub, sup
		}
		if _, ok := byID[sup]; !ok {
			continue
		}
		if _, ok := byID[sub]; !ok || sup == sub {
			continue
		}
		candidates[sub] = append(candidates[sub], candidate{parent: sup, relation: strings.TrimSpace(l.Relation), weight: l.Weight, order: l.Order})
	}

	parent := map[string]string{}
	relation := map[string]string{}
	extra := map[string][]string{}
	for _, id := range ids {
		cs := candidates[id]
		if len(cs) == 0 {
			continue
		}
		own := affiliation(byID[id])
		sameFaction := func(c candidate) bool {
			p := byID[c.parent]
			return own != "" && (affiliation(p) == own || (p.Kind == spec.groupKind && p.Name == own))
		}
		sort.SliceStable(cs, func(a, b int) bool {
			if sa, sb := sameFaction(cs[a]), sameFaction(cs[b]); sa != sb {
				return sa
			}
			if cs[a].weight != cs[b].weight {
				return cs[a].weight > cs[b].weight
			}
			return cs[a].order < cs[b].order
		})
		parent[id] = cs[0].parent
		relation[id] = cs[0].relation
		seen := map[string]bool{cs[0].parent: true}
		for _, c := range cs[1:] {
			if !seen[c.parent] {
				seen[c.parent] = true
				extra[id] = append(extra[id], c.parent)
			}
		}
	}

	// Anyone still without a superior hangs from their faction; a faction
	// named on characters but missing from the graph gets a virtual root.
	virtual := map[string]*LineageNode{}
	below := func(anc, id string) bool { // is id at or below anc?
		for v := id; v != ""; v = parent[v] {
			if v == anc {
				return true
			}
		}
		return false
	}
	attachToFaction := func(id string) {
		f := affiliation(byID[id])
		if f == "" {
			return
		}
		if fid, ok := factionByName[f]; ok {
			if fid != id && !below(id, fid) {
				parent[id] = fid
			}
			return
		}
		if byID[id].Kind == spec.groupKind {
			return // a group naming a group that does not exist: stay a root
		}
		vid := spec.virtualPrefix + f
		if virtual[vid] == nil {
			virtual[vid] = &LineageNode{ID: vid, Virtual: true, Name: f, Kind: spec.groupKind}
		}
		parent[id] = vid
	}
	for _, id := range ids {
		if _, ok := parent[id]; !ok {
			attachToFaction(id)
		}
	}

	// A loop of superiors (or of factions naming each other) cannot be a
	// tree: drop one link of each, and let that node fall back to its
	// faction where that does not close a new loop.
	cycles := breakCycles(ids, parent, relation)
	for _, loop := range cycles {
		attachToFaction(loop[0])
	}

	// Build the nodes and attach children.
	nodes := map[string]*LineageNode{}
	for _, id := range ids {
		e := byID[id]
		nodes[id] = &LineageNode{
			ID: id, Name: e.Name, Kind: e.Kind, Faction: e.Faction,
			Relation: relation[id], ExtraParents: nonNil(extra[id]), Children: []*LineageNode{},
		}
	}
	for vid, v := range virtual {
		v.Children = []*LineageNode{}
		v.ExtraParents = []string{}
		nodes[vid] = v
	}
	var unassigned *LineageNode
	var roots []*LineageNode
	for _, id := range ids {
		n := nodes[id]
		if p, ok := parent[id]; ok {
			nodes[p].Children = append(nodes[p].Children, n)
			continue
		}
		if spec.orphanKind != "" && n.Kind == spec.orphanKind {
			if unassigned == nil {
				unassigned = &LineageNode{ID: UnassignedID, Virtual: true, Name: "未归属", Kind: spec.groupKind, Children: []*LineageNode{}, ExtraParents: []string{}}
			}
			unassigned.Children = append(unassigned.Children, n)
			continue
		}
		roots = append(roots, n)
	}
	var virtualRoots []*LineageNode
	for _, v := range virtual {
		virtualRoots = append(virtualRoots, v)
	}

	for _, n := range nodes {
		sortChildren(n)
	}
	if unassigned != nil {
		sortChildren(unassigned)
	}
	sortRoots(roots)
	sortRoots(virtualRoots)
	roots = append(roots, virtualRoots...)
	if unassigned != nil {
		roots = append(roots, unassigned)
	}
	if roots == nil {
		roots = []*LineageNode{}
	}

	width, depth := layoutForest(roots)
	return Lineage{Roots: roots, Cycles: cycles, Width: width, Depth: depth}
}

// breakCycles walks each chain of parents; where one loops back, the link
// out of the loop's smallest id is dropped. Returns the loops found.
func breakCycles(ids []string, parent, relation map[string]string) [][]string {
	const (
		unseen = iota
		onPath
		done
	)
	state := map[string]int{}
	cycles := [][]string{}
	for _, start := range ids {
		var path []string
		v := start
		for v != "" && state[v] == unseen {
			state[v] = onPath
			path = append(path, v)
			v = parent[v]
		}
		if v != "" && state[v] == onPath {
			i := 0
			for path[i] != v {
				i++
			}
			loop := append([]string(nil), path[i:]...)
			sort.Strings(loop)
			delete(parent, loop[0])
			delete(relation, loop[0])
			cycles = append(cycles, loop)
		}
		for _, p := range path {
			state[p] = done
		}
	}
	return cycles
}

func subtreeSize(n *LineageNode) int {
	s := 1
	for _, c := range n.Children {
		s += subtreeSize(c)
	}
	return s
}

// sortChildren: factions before characters, then by name, then id.
func sortChildren(n *LineageNode) {
	sort.SliceStable(n.Children, func(a, b int) bool {
		x, y := n.Children[a], n.Children[b]
		if (x.Kind == "faction") != (y.Kind == "faction") {
			return x.Kind == "faction"
		}
		if x.Name != y.Name {
			return x.Name < y.Name
		}
		return x.ID < y.ID
	})
}

// sortRoots: biggest tree first, then by name.
func sortRoots(roots []*LineageNode) {
	sort.SliceStable(roots, func(a, b int) bool {
		sa, sb := subtreeSize(roots[a]), subtreeSize(roots[b])
		if sa != sb {
			return sa > sb
		}
		if roots[a].Name != roots[b].Name {
			return roots[a].Name < roots[b].Name
		}
		return roots[a].ID < roots[b].ID
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
