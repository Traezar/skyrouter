package graph

import "container/heap"

// Path is an ordered sequence of waypoint names and the total distance.
type Path struct {
	Nodes     []string
	TotalDist float64 // metres
}

// ── min-heap used by Dijkstra ──────────────────────────────────────────────

type nodeItem struct {
	name string
	dist float64
}

type nodeHeap []nodeItem

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x any)         { *h = append(*h, x.(nodeItem)) }
func (h *nodeHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ── min-heap used by Yen's candidate set ──────────────────────────────────

type pathHeap []Path

func (h pathHeap) Len() int            { return len(h) }
func (h pathHeap) Less(i, j int) bool  { return h[i].TotalDist < h[j].TotalDist }
func (h pathHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *pathHeap) Push(x any)         { *h = append(*h, x.(Path)) }
func (h *pathHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ── Dijkstra ───────────────────────────────────────────────────────────────

type edgeKey struct{ from, to string }

func dijkstra(g *Graph, from, to string, blockedEdges map[edgeKey]bool, blockedNodes map[string]bool) *Path {
	dist := map[string]float64{from: 0}
	prev := map[string]string{}
	h := &nodeHeap{{from, 0}}
	heap.Init(h)

	for h.Len() > 0 {
		cur := heap.Pop(h).(nodeItem)

		if cur.dist > dist[cur.name] {
			continue // stale entry
		}
		if cur.name == to {
			path := []string{}
			for n := to; ; n = prev[n] {
				path = append([]string{n}, path...)
				if n == from {
					break
				}
			}
			return &Path{Nodes: path, TotalDist: cur.dist}
		}

		for _, e := range g.Neighbours(cur.name) {
			if blockedNodes[e.To] || blockedEdges[edgeKey{cur.name, e.To}] {
				continue
			}
			nd := cur.dist + e.Distance
			if d, ok := dist[e.To]; !ok || nd < d {
				dist[e.To] = nd
				prev[e.To] = cur.name
				heap.Push(h, nodeItem{e.To, nd})
			}
		}
	}
	return nil
}

// ── Yen's k-shortest loopless paths ───────────────────────────────────────

// Yen returns up to k shortest loopless paths between from and to.
// Returns nil if no path exists.
func Yen(g *Graph, from, to string, k int) []Path {
	first := dijkstra(g, from, to, nil, nil)
	if first == nil {
		return nil
	}

	results := []Path{*first}
	candidates := &pathHeap{}
	heap.Init(candidates)

	for len(results) < k {
		prev := results[len(results)-1]

		for i := 0; i < len(prev.Nodes)-1; i++ {
			spurNode := prev.Nodes[i]
			rootPath := prev.Nodes[:i+1]

			blockedEdges := map[edgeKey]bool{}
			blockedNodes := map[string]bool{}

			// Block edges used by existing results that share this root prefix.
			for _, p := range results {
				if len(p.Nodes) > i+1 && sliceEq(p.Nodes[:i+1], rootPath) {
					blockedEdges[edgeKey{p.Nodes[i], p.Nodes[i+1]}] = true
				}
			}
			// Block all nodes in the root path except the spur node.
			for _, n := range rootPath[:len(rootPath)-1] {
				blockedNodes[n] = true
			}

			spur := dijkstra(g, spurNode, to, blockedEdges, blockedNodes)
			if spur == nil {
				continue
			}

			candidate := Path{
				Nodes:     append(append([]string{}, rootPath...), spur.Nodes[1:]...),
				TotalDist: rootPathDist(g, rootPath) + spur.TotalDist,
			}
			if !pathInHeap(candidates, candidate.Nodes) {
				heap.Push(candidates, candidate)
			}
		}

		if candidates.Len() == 0 {
			break
		}
		results = append(results, heap.Pop(candidates).(Path))
	}

	return results
}

// ── helpers ────────────────────────────────────────────────────────────────

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rootPathDist(g *Graph, nodes []string) float64 {
	total := 0.0
	for i := 0; i < len(nodes)-1; i++ {
		for _, e := range g.Neighbours(nodes[i]) {
			if e.To == nodes[i+1] {
				total += e.Distance
				break
			}
		}
	}
	return total
}

func pathInHeap(h *pathHeap, nodes []string) bool {
	for _, p := range *h {
		if sliceEq(p.Nodes, nodes) {
			return true
		}
	}
	return false
}
