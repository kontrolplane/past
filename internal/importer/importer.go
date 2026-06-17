// Package importer turns a CSV or XML export into pass entries. It only parses
// into a neutral Entry list; writing (encryption) stays in the pass package.
//
// CSV: a header row with a required `name` (or `path`) column and an optional
// `password` column; every other non-empty cell becomes a `key: value`
// metadata line, in column order.
//
// XML: <store><entry name="folder/x"><password>…</password><user>…</user>…
// </entry></store>. A <password> child is the first line; the name may be an
// attribute or a <name> child; any other child element becomes metadata.
package importer

import (
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// KV is one metadata line.
type KV struct{ Key, Value string }

// Entry is a parsed, store-bound record: a path, the first-line password, and
// ordered metadata lines.
type Entry struct {
	Name     string
	Password string
	Meta     []KV
}

// Content renders the entry in pass's layout: password on the first line, then
// `key: value` metadata lines.
func (e Entry) Content() string {
	var b strings.Builder
	b.WriteString(e.Password)
	b.WriteByte('\n')
	for _, m := range e.Meta {
		b.WriteString(m.Key + ": " + m.Value + "\n")
	}
	return b.String()
}

// Parse dispatches on the file extension.
func Parse(filename string, r io.Reader) ([]Entry, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		return ParseCSV(r)
	case ".xml":
		return ParseXML(r)
	default:
		return nil, fmt.Errorf("unsupported file type %q (use .csv or .xml)", filepath.Ext(filename))
	}
}

// ParseCSV reads the header-plus-rows form described in the package doc.
func ParseCSV(r io.Reader) ([]Entry, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	header := rows[0]
	nameIdx, pwIdx := -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "name", "path":
			nameIdx = i
		case "password":
			pwIdx = i
		}
	}
	if nameIdx < 0 {
		return nil, fmt.Errorf("csv: missing required 'name' (or 'path') column")
	}

	var entries []Entry
	for _, row := range rows[1:] {
		if nameIdx >= len(row) || strings.TrimSpace(row[nameIdx]) == "" {
			continue
		}
		e := Entry{Name: strings.TrimSpace(row[nameIdx])}
		if pwIdx >= 0 && pwIdx < len(row) {
			e.Password = row[pwIdx]
		}
		for i, h := range header {
			if i == nameIdx || i == pwIdx || i >= len(row) {
				continue
			}
			if v := strings.TrimSpace(row[i]); v != "" {
				e.Meta = append(e.Meta, KV{strings.TrimSpace(h), v})
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

type xmlStore struct {
	Entries []xmlEntry `xml:"entry"`
}

type xmlEntry struct {
	Name   string     `xml:"name,attr"`
	Fields []xmlField `xml:",any"`
}

type xmlField struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// ParseXML reads the <store><entry>… form described in the package doc.
func ParseXML(r io.Reader) ([]Entry, error) {
	var s xmlStore
	if err := xml.NewDecoder(r).Decode(&s); err != nil {
		return nil, err
	}
	var entries []Entry
	for _, xe := range s.Entries {
		e := Entry{Name: strings.TrimSpace(xe.Name)}
		for _, f := range xe.Fields {
			key, val := f.XMLName.Local, strings.TrimSpace(f.Value)
			switch {
			case strings.EqualFold(key, "password"):
				e.Password = val
			case strings.EqualFold(key, "name"):
				if e.Name == "" {
					e.Name = val
				}
			case val != "":
				e.Meta = append(e.Meta, KV{key, val})
			}
		}
		if e.Name == "" {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}
