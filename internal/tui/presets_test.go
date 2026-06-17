package tui

import "testing"

// submitForm walks preset.fields and advances a genInputs cursor for every
// field except a generated secret; startForm builds genInputs from
// preset.editable(). These must agree, and each preset needs exactly one name
// field and at most one generated secret, or the form indexes go out of bounds.
func TestPresetInvariants(t *testing.T) {
	for _, p := range presets {
		consumed, names, gens := 0, 0, 0
		for _, f := range p.fields {
			switch {
			case f.kind == fSecret && f.gen:
				gens++ // no input consumed
			default:
				consumed++
			}
			if f.kind == fName {
				names++
			}
		}
		if consumed != len(p.editable()) {
			t.Errorf("%s: editable()=%d but submitForm consumes %d inputs", p.name, len(p.editable()), consumed)
		}
		if names != 1 {
			t.Errorf("%s: %d name fields, want exactly 1", p.name, names)
		}
		if gens > 1 {
			t.Errorf("%s: %d generated secrets, want at most 1", p.name, gens)
		}
	}
}
