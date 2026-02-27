// Command routing demonstrates SAD-based semantic routing. It builds
// several model descriptors, creates a mock routing table, and resolves
// a request SAD to the best matching node with scoring.
//
// Usage:
//
//	go run ./strandapi/examples/routing
package main

import (
	"fmt"
	"math"
	"sort"

	"github.com/strand-protocol/strand/strandapi/pkg/sad"
)

// node represents a registered model node in the routing table.
type node struct {
	Name       string
	Descriptor *sad.SAD
}

// scoredNode pairs a node with its resolution score.
type scoredNode struct {
	Node  node
	Score float64
}

// resolve finds the best matching nodes for a request SAD, using weighted
// multi-constraint scoring per CLAUDE.md §2.2.
func resolve(request *sad.SAD, nodes []node) []scoredNode {
	// Default weights from the spec:
	//   CAPABILITY=0.3, LATENCY=0.25, COST=0.2, CONTEXT_WINDOW=0.15, TRUST=0.1
	const (
		wCap     = 0.30
		wLatency = 0.25
		wCost    = 0.20
		wCtx     = 0.15
		wTrust   = 0.10
	)

	var results []scoredNode

	for _, n := range nodes {
		// Hard constraint: context window must meet request minimum.
		if request.ContextWindow > 0 && n.Descriptor.ContextWindow < request.ContextWindow {
			continue // disqualified
		}

		// Capability score: fraction of requested capabilities present.
		capScore := 0.0
		if request.Capabilities != 0 {
			matched := request.Capabilities & n.Descriptor.Capabilities
			total := popcount(request.Capabilities)
			if total > 0 {
				capScore = float64(popcount(matched)) / float64(total)
			}
		} else {
			capScore = 1.0 // no requirements → full score
		}

		// Latency score: how well the node meets the SLA (lower is better).
		latScore := 1.0
		if request.LatencySLA > 0 && n.Descriptor.LatencySLA > 0 {
			ratio := float64(n.Descriptor.LatencySLA) / float64(request.LatencySLA)
			if ratio <= 1.0 {
				latScore = 1.0
			} else {
				latScore = math.Max(0.0, 1.0-((ratio-1.0)*0.5))
			}
		}

		// Context window score: bonus for exceeding minimum.
		ctxScore := 1.0
		if request.ContextWindow > 0 {
			ratio := float64(n.Descriptor.ContextWindow) / float64(request.ContextWindow)
			ctxScore = math.Min(ratio, 2.0) / 2.0 // cap at 1.0 for 2x
		}

		// Cost and trust are not modeled in the SAD struct yet, so use 1.0.
		costScore := 1.0
		trustScore := 1.0

		total := wCap*capScore + wLatency*latScore + wCost*costScore + wCtx*ctxScore + wTrust*trustScore

		results = append(results, scoredNode{Node: n, Score: total})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

func popcount(v uint32) uint32 {
	var count uint32
	for v != 0 {
		count += v & 1
		v >>= 1
	}
	return count
}

func capString(caps uint32) string {
	var names []string
	if caps&sad.TextGen != 0 {
		names = append(names, "TextGen")
	}
	if caps&sad.CodeGen != 0 {
		names = append(names, "CodeGen")
	}
	if caps&sad.Embedding != 0 {
		names = append(names, "Embedding")
	}
	if caps&sad.ImageGen != 0 {
		names = append(names, "ImageGen")
	}
	if caps&sad.AudioGen != 0 {
		names = append(names, "AudioGen")
	}
	if caps&sad.ToolUse != 0 {
		names = append(names, "ToolUse")
	}
	if caps&sad.Vision != 0 {
		names = append(names, "Vision")
	}
	if len(names) == 0 {
		return "(none)"
	}
	result := names[0]
	for _, n := range names[1:] {
		result += " | " + n
	}
	return result
}

func main() {
	fmt.Println("=== Strand Protocol — SAD-Based Semantic Routing ===")
	fmt.Println()

	// ---------------------------------------------------------------
	// 1. Register model nodes in the routing table
	// ---------------------------------------------------------------
	nodes := []node{
		{
			Name: "gpt4-node",
			Descriptor: &sad.SAD{
				Version:       1,
				ModelType:     "llm",
				Capabilities:  sad.TextGen | sad.CodeGen | sad.ToolUse,
				ContextWindow: 128000,
				LatencySLA:    200,
			},
		},
		{
			Name: "vision-node",
			Descriptor: &sad.SAD{
				Version:       1,
				ModelType:     "diffusion",
				Capabilities:  sad.ImageGen | sad.Vision,
				ContextWindow: 4096,
				LatencySLA:    2000,
			},
		},
		{
			Name: "embedding-node",
			Descriptor: &sad.SAD{
				Version:       1,
				ModelType:     "embedding",
				Capabilities:  sad.Embedding,
				ContextWindow: 8192,
				LatencySLA:    50,
			},
		},
		{
			Name: "code-fast-node",
			Descriptor: &sad.SAD{
				Version:       1,
				ModelType:     "llm",
				Capabilities:  sad.TextGen | sad.CodeGen,
				ContextWindow: 32000,
				LatencySLA:    100,
			},
		},
	}

	fmt.Println("Registered nodes:")
	for _, n := range nodes {
		fmt.Printf("  %-20s  type=%-10s  caps=%s  ctx=%dk  lat=%dms\n",
			n.Name, n.Descriptor.ModelType, capString(n.Descriptor.Capabilities),
			n.Descriptor.ContextWindow/1000, n.Descriptor.LatencySLA)
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// 2. Resolve request SADs against the routing table
	// ---------------------------------------------------------------
	requests := []struct {
		name string
		sad  *sad.SAD
	}{
		{
			name: "Code generation with large context",
			sad: &sad.SAD{
				Capabilities:  sad.TextGen | sad.CodeGen,
				ContextWindow: 64000,
				LatencySLA:    500,
			},
		},
		{
			name: "Fast embedding lookup",
			sad: &sad.SAD{
				Capabilities: sad.Embedding,
				LatencySLA:   100,
			},
		},
		{
			name: "Image generation",
			sad: &sad.SAD{
				Capabilities: sad.ImageGen,
				LatencySLA:   5000,
			},
		},
	}

	for _, r := range requests {
		fmt.Printf("--- Request: %s ---\n", r.name)
		fmt.Printf("  Required: caps=%s  ctx>=%dk  lat<=%dms\n",
			capString(r.sad.Capabilities), r.sad.ContextWindow/1000, r.sad.LatencySLA)

		results := resolve(r.sad, nodes)
		if len(results) == 0 {
			fmt.Println("  Result: no matching nodes")
		} else {
			fmt.Println("  Results (best first):")
			for i, s := range results {
				fmt.Printf("    %d. %-20s  score=%.3f\n", i+1, s.Node.Name, s.Score)
			}
			fmt.Printf("  -> Routed to: %s (score %.3f)\n", results[0].Node.Name, results[0].Score)
		}
		fmt.Println()
	}

	fmt.Println("=== Routing demo complete ===")
}
