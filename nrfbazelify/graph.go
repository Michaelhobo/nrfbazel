package nrfbazelify

import (
	"fmt"
	"log"
	"os"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
	"github.com/google/uuid"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

// NewDependencyGraph creates a new DependencyGraph.
func NewDependencyGraph(sdkDir, workspaceDir string) *DependencyGraph {
  return &DependencyGraph{
    sdkDir: sdkDir,
    workspaceDir: workspaceDir,
    labelToID: make(map[string]int64),
    idToLabel: make(map[int64]string),
    fileNameToLabel: make(map[string]*labelResolver),
    graph: simple.NewDirectedGraph(),
  }
}

// DependencyGraph is a Bazel dependency graph used to resolve conflicts and fix cyclic dependencies.
type DependencyGraph struct {
  sdkDir, workspaceDir string
  nextID int64
  labelToID map[string]int64 // label.String() -> node ID
  idToLabel map[int64]string // node ID -> label.String()
  fileNameToLabel map[string]*labelResolver // file name (base only) -> indexed file
  graph *simple.DirectedGraph
}

// OutputDOTGraph outputs the graph's contents as a DOT graph.
// The graph is written to the file at the given path.
func (d *DependencyGraph) OutputDOTGraph(path string) error {
  out, err := dot.Marshal(d.graph, "Dependencies", "", "")
  if err != nil {
    return fmt.Errorf("dot.Marshal: %v", err)
  }
  if err := os.WriteFile(path, out, 0640); err != nil {
    return fmt.Errorf("WriteFile(%q): %v", path, err)
  }
  return nil
}

func (d *DependencyGraph) Nodes() []Node {
  var out []Node
  nodes := d.graph.Nodes()
  for nodes.Next() {
    out = append(out, nodes.Node().(Node))
  }
  return out
}

// Node fetches the node corresponding to the label.
// Returns nil if not found.
func (d *DependencyGraph) Node(label *bazel.Label) Node {
  n := d.graph.Node(d.labelToID[label.String()])
  if n == nil {
    return nil
  }
  return n.(Node)
}

// NodesWithFile searches for all nodes containing the given file name.
// The name should be the base name of the file, without the full path.
func (d *DependencyGraph) NodesWithFile(name string) []Node {
  if d.fileNameToLabel[name] == nil {
    return nil
  }
  var out []Node
  for _, label := range d.fileNameToLabel[name].validLabels() {
    out = append(out, d.Node(label))
  }
  return out
}

// IsFileOverridden checks if the file with the given name has an override.
func (d *DependencyGraph) IsFileOverridden(name string) bool {
  return d.fileNameToLabel[name] != nil && d.fileNameToLabel[name].override != nil
}

// AddLibraryNode adds a node that represents a cc_library rule.
// If includeOwnDir is true, we add our current directory to the includes cc_library field.
func (d *DependencyGraph) AddLibraryNode(label *bazel.Label, srcs, hdrs []string, includes []string) error {
  // If an override node is taking up our label, delete it.
  if _, overrideExists := d.Node(label).(*OverrideNode); overrideExists {
    if err := d.deleteNode(label); err != nil {
      return fmt.Errorf("found override node, deleteNode(%q): %v", label, err)
    }
  }

  nodeID, err := d.nodeID(label)
  if err != nil {
    return err
  }

  var indexFiles []string
  indexFiles = append(indexFiles, srcs...)
  indexFiles = append(indexFiles, hdrs...)
  d.indexFiles(label, indexFiles)

  d.graph.AddNode(&LibraryNode{
    id: nodeID,
    label: label,
    Srcs: srcs,
    Hdrs: hdrs,
    Includes: includes,
  })
  return nil
}

// AddRemapNode adds a node that represents a remapped rule.
func (d *DependencyGraph) AddRemapNode(label *bazel.Label, fileName string, labelSetting *buildfile.LabelSetting) error {
  // If an override node is taking up our label, delete it.
  if _, overrideExists := d.Node(label).(*OverrideNode); overrideExists {
    if err := d.deleteNode(label); err != nil {
      return fmt.Errorf("found override node, deleteNode(%q): %v", label, err)
    }
  }

  // Set it as the override
  if d.fileNameToLabel[fileName] == nil {
    d.fileNameToLabel[fileName] = newLabelResolver()
  }
  resolver := d.fileNameToLabel[fileName]
  if resolver.override != nil {
    return fmt.Errorf("override for %q already exists(%q), can't add remap %q", fileName, resolver.override, label)
  }
  resolver.override = label

  nodeID, err := d.nodeID(label)
  if err != nil {
    return err
  }

  d.graph.AddNode(&RemapNode{
    id: nodeID,
    label: label,
    LabelSetting: labelSetting,
  })
  return nil
}

// AddOverrideNode adds a node that represents a target_override from bazelifyrc.
func (d *DependencyGraph) AddOverrideNode(fileName string, label *bazel.Label) error {
  if d.fileNameToLabel[fileName] == nil {
    d.fileNameToLabel[fileName] = newLabelResolver()
  }
  resolver := d.fileNameToLabel[fileName]
  if resolver.override != nil {
    return fmt.Errorf("override for %q already exists(%q), can't add second override %q", fileName, resolver.override, label)
  }
  resolver.override = label

  nodeID, err := d.nodeID(label)
  if err != nil {
    // If the label is already taken, just skip it.
    return nil
  }
  d.graph.AddNode(&OverrideNode{
    id: nodeID,
    label: label,
  })
  return nil
}

// AddGroupNode adds an empty group node that represents a set of nodes.
func (d *DependencyGraph) AddGroupNode() (*GroupNode, error) {
  label, err := bazel.NewLabel(d.sdkDir, uuid.NewString(), d.workspaceDir)
  if err != nil {
    return nil, fmt.Errorf("bazel.NewLabel: %v", err)
  }
  nodeID, err := d.nodeID(label)
  if err != nil {
    return nil, fmt.Errorf("d.nodeID(%q): %v", label, err)
  }
  node := &GroupNode{
    id: nodeID,
    label: label,
  }
  d.graph.AddNode(node)
  return node, nil	
}

// AddDependency adds a dependency from src to dst
func (d *DependencyGraph) AddDependency(src, dst *bazel.Label) error {
  srcID := d.labelToID[src.String()]
  dstID := d.labelToID[dst.String()]
  if srcID == 0 {
    return fmt.Errorf("%q not in graph", src)
  }
  if dstID == 0 {
    return fmt.Errorf("%q not in graph", dst)
  }
  if d.graph.HasEdgeFromTo(srcID, dstID) {
    return nil
  }
  srcNode := d.graph.Node(srcID).(Node)
  dstNode := d.graph.Node(dstID).(Node)
  cyclicEdges := d.edgesFromTo(dstNode, srcNode)
  if len(cyclicEdges) != 0 {
    if err := d.mergeCycle(cyclicEdges); err != nil {
      return fmt.Errorf("mergeCycle: %v", err)
    }
    return nil
  }
  edge := d.graph.NewEdge(srcNode, dstNode)
  d.graph.SetEdge(edge)
  return nil
}

// Dependencies returns all nodes that are dependencies of node.
func (d *DependencyGraph) Dependencies(label *bazel.Label) []Node {
  var out []Node
  nodes := d.graph.From(d.Node(label).ID())
  for nodes.Next() {
    out = append(out, nodes.Node().(Node))
  }
  return out
}

func (d *DependencyGraph) edgesFromTo(src, dst Node) ([]graph.Edge) {
  var edges []graph.Edge
  nodes := d.graph.From(src.ID())
  for nodes.Next() {
    next := nodes.Node()
    if next.ID() == dst.ID() {
      edges = append(edges, d.graph.NewEdge(src, dst))
      continue
    }
    childEdges := d.edgesFromTo(nodes.Node().(Node), dst)
    if len(childEdges) == 0 {
      continue
    }
    edges = append(edges, d.graph.NewEdge(src, next))
    edges = append(edges, childEdges...)
  }
  return edges
}

func (d *DependencyGraph) mergeCycle(cyclicEdges []graph.Edge) error {
  for _, edge := range cyclicEdges {
    d.graph.RemoveEdge(edge.From().ID(), edge.To().ID())
  }

  nodeIDs := make(map[int64]bool)
  for _, edge := range cyclicEdges {
    nodeIDs[edge.From().ID()] = true
    nodeIDs[edge.To().ID()] = true
  }

  groupNode := d.findGroupNode(nodeIDs)
  if groupNode == nil {
    node, err := d.AddGroupNode()
    if err != nil {
      return fmt.Errorf("AddGroupNode: %v", err)
    }
    groupNode = node
  }

  for nodeID := range nodeIDs {
    if nodeID == groupNode.ID() {
      continue
    }
    node := d.graph.Node(nodeID).(Node)

    var files []string
    switch n := node.(type) {
    case *GroupNode:
      files = append(files, n.Srcs...)
      files = append(files, n.Hdrs...)
    case *LibraryNode:
      files = append(files, n.Srcs...)
      files = append(files, n.Hdrs...)
    default:
      return fmt.Errorf("node %q not supported", n.Label())
    }
    d.deindexFiles(node.Label(), files)
    d.indexFiles(groupNode.Label(), files)

    if err := groupNode.Absorb(node); err != nil {
      return fmt.Errorf("groupNode.Absorb(%q): %v", node.Label(), err)
    }
    fromNodes := d.graph.From(nodeID)
    for fromNodes.Next() {
      d.graph.RemoveEdge(nodeID, fromNodes.Node().ID())
      if fromNodes.Node().ID() == groupNode.ID() {
        continue
      }
      d.graph.SetEdge(d.graph.NewEdge(groupNode, fromNodes.Node()))
    }
    toNodes := d.graph.To(nodeID)
    for toNodes.Next() {
      d.graph.RemoveEdge(toNodes.Node().ID(), nodeID)
      if toNodes.Node().ID() == groupNode.ID() {
        continue
      }
      d.graph.SetEdge(d.graph.NewEdge(toNodes.Node(), groupNode))
    }
  }

  for nodeID := range nodeIDs {
    if nodeID == groupNode.ID() {
      continue
    }
    _, isGroupNode := d.graph.Node(nodeID).(*GroupNode)
    if !isGroupNode {
      continue
    }
    d.graph.RemoveNode(nodeID)
    delete(nodeIDs, nodeID)
  }

  for nodeID := range nodeIDs {
    if nodeID == groupNode.ID() {
      continue
    }
    node, isLibraryNode := d.graph.Node(nodeID).(*LibraryNode)
    if !isLibraryNode {
      return fmt.Errorf("node %q must be a library node", d.graph.Node(nodeID).(Node).Label())
    }
    node.IsPointer = true
    d.graph.SetEdge(d.graph.NewEdge(node, groupNode))
  }

  return nil
}

    // TODO: Does absorbing pointer nodes work?
    // TODO: I don't think HasEdgeFromTo is what we want

func (d *DependencyGraph) findGroupNode(nodeIDs map[int64]bool) *GroupNode {
  for nodeID := range nodeIDs {
    switch n := d.graph.Node(nodeID).(type) {
    case *GroupNode:
      return n
    }
  }
  return nil
}

func (d *DependencyGraph) nodeID(label *bazel.Label) (int64, error) {
  if d.labelToID[label.String()] != 0 {
    return 0, fmt.Errorf("%q already part of graph", label)
  }
  // We increment nextID first to avoid using the zero value of int64.
  d.nextID++
  d.labelToID[label.String()] = d.nextID
  d.idToLabel[d.nextID] = label.String()
  return d.nextID, nil
}

func (d *DependencyGraph) indexFiles(label *bazel.Label, fileNames []string) {
  for _, fileName := range fileNames {
    if d.fileNameToLabel[fileName] == nil {
      d.fileNameToLabel[fileName] = newLabelResolver()
    }
    d.fileNameToLabel[fileName].possible[label.String()] = true
  }
}

func (d *DependencyGraph) deindexFiles(label *bazel.Label, fileNames []string) {
  for _, fileName := range fileNames {
    delete(d.fileNameToLabel[fileName].possible, label.String())
    if d.fileNameToLabel[fileName].empty() {
      delete(d.fileNameToLabel, fileName)
    }
  }
}

func (d *DependencyGraph) deleteNode(label *bazel.Label) error {
  nodeID := d.labelToID[label.String()]
  if nodeID == 0 {
    return fmt.Errorf("%q not part of graph", label)
  }
  
  node := d.graph.Node(nodeID)
  var indexFiles []string
  switch n := node.(type) {
  case *LibraryNode:
    indexFiles = append(indexFiles, n.Srcs...)
    indexFiles = append(indexFiles, n.Hdrs...)
  case *GroupNode:
    indexFiles = append(indexFiles, n.Srcs...)
    indexFiles = append(indexFiles, n.Hdrs...)
  default:
    log.Fatalf("unknown graph.Node type with node %v", node)
  }

  d.deindexFiles(label, indexFiles)
  d.graph.RemoveNode(nodeID)

  delete(d.labelToID, label.String())
  delete(d.idToLabel, nodeID)
  return nil
}

func newLabelResolver() *labelResolver {
  return &labelResolver{
    possible: make(map[string]bool),
  }
}

type labelResolver struct {
  override *bazel.Label
  possible map[string]bool // label.String() -> true
}

func (l *labelResolver) validLabels() []*bazel.Label {
  if l.override != nil {
    return []*bazel.Label{l.override}
  }
  var out []*bazel.Label
  for label := range l.possible {
    l, err := bazel.ParseLabel(label)
    if err != nil {
      log.Fatalf("bazel.ParseLabel(%q): %v", label, err)
    }
    out = append(out, l)
  }
  return out
}

func (l *labelResolver) empty() bool {
  return l.override == nil && len(l.possible) == 0
}