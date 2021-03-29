package bazel

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	labelRegexp = regexp.MustCompile(`^//([\w/]*)(?::(\w+))?$`)
	relativeLabelRegexp = regexp.MustCompile(`^:(\w+)$`)
)

func JoinLabelStrings(labels []*Label, separator string) string {
	var out string
	for i, label := range labels {
		if i > 0 {
			out += separator
		}
		out += label.String()
	}
	return out
}

// NewLabel creates a new label.
func NewLabel(absDir, name, workspaceDir string) (*Label, error) {
	if !filepath.IsAbs(absDir) {
		return nil, fmt.Errorf("%q must be an absolute path", absDir)
	}
	if !strings.HasPrefix(absDir, workspaceDir) {
		return nil, fmt.Errorf("%q must be in %q", absDir, workspaceDir)
	}
	dir, err := filepath.Rel(workspaceDir, absDir)
	if err != nil {
		return nil, fmt.Errorf("filepath.Rel(%q, %q): %v", workspaceDir, absDir, err)
	}
	return &Label{
		dir: dir,
		name: name,
	}, nil
}

func ParseLabel(label string) (*Label, error) {
	capture := labelRegexp.FindStringSubmatch(label)
	if capture == nil {
		return nil, fmt.Errorf("%q does not match %q", label, labelRegexp)
	}
	dir := capture[1]
	name := capture[2]
	if name == "" {
		if dir == "" {
			return nil, fmt.Errorf("%q returned an empty regexp capture", label)
		}
		name = filepath.Base(dir)
	}
	return &Label{
		dir: dir,
		name: name,
	}, nil
}

func ParseRelativeLabel(other *Label, label string) (*Label, error) {
	if other == nil {
		return nil, fmt.Errorf("other label is nil")
	}
	capture := relativeLabelRegexp.FindStringSubmatch(label)
	if len(capture) == 0 {
		return ParseLabel(label)
	}
	name := capture[1]
	if name == "" {
		return nil, fmt.Errorf("%q returned an empty regexp capture", label)
	}	
	return &Label{
		dir: other.dir,
		name: name,
	}, nil
}

// Label is a Bazel BUILD label.
type Label struct {
	// Relative dir from 
	dir string
	name string
}

// Name returns the label's name.
func (l *Label) Name() string {
	return l.name
}

// Dir returns the directory the label belongs in.
func (l *Label) Dir() string {
	return l.dir
}

func (l *Label) String() string {
	out := fmt.Sprintf("//%s", l.dir)
	if filepath.Base(l.dir) != l.name {
		out = fmt.Sprintf("%s:%s", out, l.name)
	}
	return out
}

// RelativeTo generates the label string relative to another label.
func (l *Label) RelativeTo(other *Label) string {
	if l.dir != other.dir {
		return l.String()
	}
	return fmt.Sprintf(":%s", l.name)
}