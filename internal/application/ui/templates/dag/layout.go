package uidag

import (
	appsession "github.com/mwantia/forge/internal/application/session"
)

// Node is a positioned DAG node for the SVG canvas.
type Node struct {
	ID    string
	X, Y  float64
	Kind  string // "user", "assistant", "tool", "system"
	Label string
	Hash  string
}

// Edge connects two nodes by hash.
type Edge struct {
	From, To string
}

// BuildLayout computes top-down positions for the full-page DAG SVG.
// Nodes are placed in message order with ref labels on tip hashes.
func BuildLayout(messages []*appsession.Message, refs map[string]string) ([]Node, []Edge) {
	return buildLayout(messages, refs, 440, 40, 60)
}

// BuildMiniLayout computes compressed positions for the sidebar thumbnail.
func BuildMiniLayout(messages []*appsession.Message, refs map[string]string) ([]Node, []Edge) {
	return buildLayout(messages, refs, 110, 18, 28)
}

func buildLayout(messages []*appsession.Message, refs map[string]string, xCenter float64, yStart, yStep int) ([]Node, []Edge) {
	if len(messages) == 0 {
		return nil, nil
	}

	refsByHash := make(map[string][]string, len(refs))
	for name, hash := range refs {
		if name != "HEAD" {
			refsByHash[hash] = append(refsByHash[hash], name)
		}
	}

	nodes := make([]Node, 0, len(messages))
	edges := make([]Edge, 0, len(messages)-1)

	for i, msg := range messages {
		label := ""
		if rnames, ok := refsByHash[msg.Hash]; ok {
			label = rnames[0]
		}
		nodes = append(nodes, Node{
			ID:    msg.Hash,
			X:     xCenter,
			Y:     float64(yStart + i*yStep),
			Kind:  msg.Role,
			Label: label,
			Hash:  msg.Hash,
		})
		if i > 0 {
			edges = append(edges, Edge{
				From: messages[i-1].Hash,
				To:   msg.Hash,
			})
		}
	}

	return nodes, edges
}
