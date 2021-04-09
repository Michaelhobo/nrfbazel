package bazel

import "testing"

func TestLabel_String(t *testing.T) {
  tests := map[string]struct{
    label *Label
    want string
  }{
    "nominal": {
      label: &Label{
        dir: "something/out/there",
        name: "aliens",
      },
      want: "//something/out/there:aliens",
    },
    "name matches directory": {
      label: &Label{
        dir: "something/out/there",
        name: "there",
      },
      want: "//something/out/there",
    },
    "no directory": {
      label: &Label{
        name: "aliens",
      },
      want: "//:aliens",
    },
  }
  for name, test := range tests {
    t.Run(name, func(t *testing.T) {
      if got := test.label.String(); got != test.want {
        t.Errorf("%v String()=%q, want %q", test.label, got, test.want)
      }
    })
  }
}

func TestLabel_RelativeTo(t *testing.T) {
  tests := map[string]struct{
    label *Label
    other *Label
    want string
  }{
    "same directory": {
      label: &Label{
        dir: "something/out/there",
        name: "aliens",
      },
      other: &Label{
        dir: "something/out/there",
        name: "stars",
      },
      want: ":aliens",
    },
    "different directory": {
      label: &Label{
        dir: "something/out/there",
        name: "aliens",
      },
      other: &Label{
        dir: "on/earth",
        name: "humans",
      },
      want: "//something/out/there:aliens",
    },
    "no directory": {
      label: &Label{
        name: "aliens",
      },
      other: &Label{
        name: "humans",
      },
      want: ":aliens",
    },
  }
  for name, test := range tests {
    t.Run(name, func(t *testing.T) {
      if got := test.label.RelativeTo(test.other); got != test.want {
        t.Errorf("%v RelativeTo(%v)=%q, want %q", test.label, test.other, got, test.want)
      }
    })
  }
}

func TestLabel_ParseLabel(t *testing.T) {
  tests := map[string]struct{
    label string
    want *Label
  }{
    "nominal": {
      label: "//something/out/there:aliens",
      want: &Label{
        dir: "something/out/there",
        name: "aliens",
      },
    },
    "name matches directory": {
      label: "//something/out/there",
      want: &Label{
        dir: "something/out/there",
        name: "there",
      },
    },
    "no directory": {
      label: "//:aliens",
      want: &Label{
        name: "aliens",
      },
    },
  }
  for name, test := range tests {
    t.Run(name, func(t *testing.T) {
      got, err := ParseLabel(test.label)
      if err != nil {
        t.Errorf("ParseLabel(%q): %v", test.label, err)
        return
      }
      if got.String() != test.want.String() {
        t.Errorf("ParseLabel(%q)=%q, want %q", test.label, got, test.want)
      }
    })
  }

}

func TestLabel_ParseRelativeLabel(t *testing.T) {
  tests := map[string]struct{
    label string
    other *Label
    want *Label
  }{
    "same directory": {
      label: ":aliens",
      other: &Label{
        dir: "something/out/there",
        name: "stars",
      },
      want: &Label{
        dir: "something/out/there",
        name: "aliens",
      },
    },
    "different directory": {
      label: "//something/out/there:aliens",
      other: &Label{
        dir: "on/earth",
        name: "humans",
      },
      want: &Label{
        dir: "something/out/there",
        name: "aliens",
      },
    },
    "no directory": {
      label: ":aliens",
      other: &Label{
        name: "humans",
      },
      want: &Label{
        name: "aliens",
      },
    },
  }
  for name, test := range tests {
    t.Run(name, func(t *testing.T) {
      got, err := ParseRelativeLabel(test.other, test.label)
      if err != nil {
        t.Errorf("ParseRelativeLabel(%q, %q): %v", test.other, test.label, err)
        return
      }
      if got.String() != test.want.String() {
        t.Errorf("ParseRelativeLabel(%q, %q)=%q, want %q", test.other, test.label, got, test.want)
      }
    })
  }
}