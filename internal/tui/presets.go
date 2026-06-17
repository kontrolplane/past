package tui

// Preset entry types for in-app creation. pass itself has no notion of types —
// these are just field templates past offers. Every entry still lands as plain
// pass text: the first content line is the secret, the rest are `key: value`.
//
// ponytail: type is not persisted (pass entries are freeform), so on reload the
// detail pane can't tell a card from a login — it labels line 0 "password"
// regardless. Storing a `type:` line would fix it if that ever matters.

type fieldKind int

const (
	fName   fieldKind = iota // the entry path/name
	fSecret                  // first content line, masked in the detail pane
	fMeta                    // a `key: value` metadata line
)

type genField struct {
	label string
	key   string // metadata key (fMeta only)
	kind  fieldKind
	gen   bool // fSecret: generate the value via pass rather than type it
}

type preset struct {
	name   string
	fields []genField
}

var presets = []preset{
	{"login", []genField{
		{label: "name", kind: fName},
		{label: "password", kind: fSecret, gen: true},
		{label: "username", key: "username", kind: fMeta},
		{label: "url", key: "url", kind: fMeta},
	}},
	{"password", []genField{
		{label: "name", kind: fName},
		{label: "password", kind: fSecret, gen: true},
	}},
	{"credit card", []genField{
		{label: "name", kind: fName},
		{label: "number", kind: fSecret},
		{label: "expiry", key: "expiry", kind: fMeta},
		{label: "cvv", key: "cvv", kind: fMeta},
		{label: "cardholder", key: "cardholder", kind: fMeta},
	}},
	{"email", []genField{
		{label: "name", kind: fName},
		{label: "password", kind: fSecret, gen: true},
		{label: "email", key: "email", kind: fMeta},
	}},
	{"server", []genField{
		{label: "name", kind: fName},
		{label: "password", kind: fSecret, gen: true},
		{label: "host", key: "host", kind: fMeta},
		{label: "username", key: "username", kind: fMeta},
	}},
	{"database", []genField{
		{label: "name", kind: fName},
		{label: "password", kind: fSecret, gen: true},
		{label: "host", key: "host", kind: fMeta},
		{label: "port", key: "port", kind: fMeta},
		{label: "username", key: "username", kind: fMeta},
		{label: "database", key: "database", kind: fMeta},
	}},
	{"api token", []genField{
		{label: "name", kind: fName},
		{label: "token", kind: fSecret}, // pasted, not generated
		{label: "url", key: "url", kind: fMeta},
	}},
	{"secure note", []genField{
		{label: "name", kind: fName},
		{label: "note", kind: fSecret},
	}},
}

// editable lists the fields that need a text input, in field order — i.e. every
// field except a generated secret. The slice index lines up with Model.genInputs.
func (p preset) editable() []genField {
	var out []genField
	for _, f := range p.fields {
		if f.kind == fSecret && f.gen {
			continue
		}
		out = append(out, f)
	}
	return out
}

// hasGen reports whether the preset generates a password (shows length/symbols).
func (p preset) hasGen() bool {
	for _, f := range p.fields {
		if f.kind == fSecret && f.gen {
			return true
		}
	}
	return false
}

// labelWidth is the column width for aligning field labels in the form.
func (p preset) labelWidth() int {
	w := len("symbols")
	for _, f := range p.fields {
		if len(f.label) > w {
			w = len(f.label)
		}
	}
	return w
}
