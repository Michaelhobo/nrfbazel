package nrfbazelify

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

var reportTemplate = template.Must(template.New("report").Parse(`Graph stats:
  Node count: {{ .NodeCount }}
  Edge count: {{ .EdgeCount }}
  Group count: {{ .GroupCount }}
`))

// NewGraphStats creates a new GraphStats instance from a snapshot of the current graph.
func NewGraphStats(conf *Config, graph *DependencyGraph) (*GraphStats, error) {
  namedGroupGraphs := make(map[string]*simple.DirectedGraph)
  for _, byLastHeader := range conf.NamedGroups {
    for _, name := range byLastHeader {
      label, err := bazel.NewLabel(conf.SDKDir, name, conf.WorkspaceDir)
      if err != nil {
        return nil, fmt.Errorf("bazel.NewLabel(%q, %q): %v", conf.SDKDir, name, err)
      }
      subGraph, err := newSubGraph(graph, label)
      if err != nil {
        return nil, fmt.Errorf("creating subgraph for named group %q: %v", name, err)
      }
      namedGroupGraphs[name] = subGraph
    }
  }
  return &GraphStats{
    NodeCount: graph.graph.Nodes().Len(),
    EdgeCount: graph.graph.Edges().Len(),
    GroupCount: len(namedGroupGraphs),
    NamedGroupGraphs: namedGroupGraphs,
  }, nil
}

// GraphStats contains stats about the dependency graph.
// It can be used to generate a report.
type GraphStats struct {
  NodeCount int
  EdgeCount int
  GroupCount int
  NamedGroupGraphs map[string]*simple.DirectedGraph // named group name -> subgraph
}

// Generates a human-readable report of the graph stats.
func (g *GraphStats) GenerateReport() string {
  var out bytes.Buffer
  reportTemplate.Execute(&out, g)
  return out.String()
}

// WriteNamedGroupGraphs writes subgraphs of all named groups as DOT graphs to the given directory.
func (g *GraphStats) WriteNamedGroupGraphs(dir string) error {
  for name, graph := range g.NamedGroupGraphs {
    path := filepath.Join(dir, fmt.Sprintf("%s.dot", name))
    out, err := dot.Marshal(graph, fmt.Sprintf("Named Group %q", name), "", "")
    if err != nil {
      return fmt.Errorf("dot.Marshal: %v", err)
    }
    if err := os.WriteFile(path, out, 0644); err != nil {
      return fmt.Errorf("WriteFile(%q): %v", path, err)
    }
  }
  return nil
}

// newSubGraph generates a graph that contains the node with the given label,
// and all directly connected nodes.
func newSubGraph(graph *DependencyGraph, label *bazel.Label) (*simple.DirectedGraph, error) {
  out := simple.NewDirectedGraph()
  node := graph.Node(label)
  if node == nil {
    return nil, fmt.Errorf("node %q is nil", label)
  }
  out.AddNode(node)
  fromNodes := graph.graph.From(node.ID())
  for fromNodes.Next() {
    fromNode := fromNodes.Node()
    out.AddNode(fromNode)
    out.SetEdge(out.NewEdge(node, fromNode))
  }
  toNodes := graph.graph.To(node.ID())
  for toNodes.Next() {
    toNode := toNodes.Node()
    out.AddNode(toNode)
    out.SetEdge(out.NewEdge(toNode, node))
  }
  return out, nil
}