package graph

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// WaypointNode holds position data for a single waypoint.
type WaypointNode struct {
	Name string
	Lat  float64
	Lon  float64
}

// Edge connects a waypoint to a neighbour with a pre-computed distance.
type Edge struct {
	To       string
	Distance float64 // metres
}

// Graph is an in-memory adjacency list loaded from the waypoint_edges table.
type Graph struct {
	nodes map[string]WaypointNode
	adj   map[string][]Edge
}

// Node returns position data for a named waypoint.
func (g *Graph) Node(name string) (WaypointNode, bool) {
	n, ok := g.nodes[name]
	return n, ok
}

// Neighbours returns outbound edges from a waypoint.
func (g *Graph) Neighbours(name string) []Edge {
	return g.adj[name]
}

// Querier is satisfied by *pgxpool.Pool.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Load reads waypoint positions and pre-built edges from the database.
// This replaces the former O(N²) in-memory Build — the heavy lifting is done
// once by migration 000006 and stored in the waypoint_edges table.
func Load(ctx context.Context, db Querier) (*Graph, error) {
	// --- waypoint positions ---
	wrows, err := db.Query(ctx, `SELECT name, latitude, longitude FROM waypoints`)
	if err != nil {
		return nil, err
	}
	defer wrows.Close()

	nodes := make(map[string]WaypointNode)
	for wrows.Next() {
		var n WaypointNode
		if err := wrows.Scan(&n.Name, &n.Lat, &n.Lon); err != nil {
			return nil, err
		}
		nodes[n.Name] = n
	}
	if err := wrows.Err(); err != nil {
		return nil, err
	}

	// --- pre-built edges ---
	erows, err := db.Query(ctx, `SELECT from_name, to_name, distance_m FROM waypoint_edges`)
	if err != nil {
		return nil, err
	}
	defer erows.Close()

	adj := make(map[string][]Edge, len(nodes))
	for erows.Next() {
		var from, to string
		var dist float64
		if err := erows.Scan(&from, &to, &dist); err != nil {
			return nil, err
		}
		adj[from] = append(adj[from], Edge{To: to, Distance: dist})
	}
	if err := erows.Err(); err != nil {
		return nil, err
	}

	return &Graph{nodes: nodes, adj: adj}, nil
}
