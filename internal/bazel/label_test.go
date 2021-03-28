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

}

func TestLabel_ParseRelativeLabel(t *testing.T) {

}