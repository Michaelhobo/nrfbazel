package nrfbazelify

import (
	"bytes"
	"text/template"
)

var reportTemplate = template.Must(template.New("report").Parse(`Graph stats:
  Node count: {{ .NodeCount }}
  Edge count: {{ .EdgeCount }}
`))

// GraphStats contains stats about the dependency graph.
// It can be used to generate a report.
type GraphStats struct {
  NodeCount int
  EdgeCount int
}

// Generates a human-readable report of the graph stats.
func (g *GraphStats) GenerateReport() string {
  var out bytes.Buffer
  reportTemplate.Execute(&out, g)
  return out.String()
}
