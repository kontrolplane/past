package importer

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	csv := "name,password,user,region\n" +
		"aws/key,secret1,demo,eu-west-1\n" +
		"wifi,secret2,,\n" + // empty cells skipped
		",orphan,,\n" // no name → skipped

	got, err := ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Name: "aws/key", Password: "secret1", Meta: []KV{{"user", "demo"}, {"region", "eu-west-1"}}},
		{Name: "wifi", Password: "secret2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCSV =\n%+v\nwant\n%+v", got, want)
	}
	if c := want[0].Content(); c != "secret1\nuser: demo\nregion: eu-west-1\n" {
		t.Errorf("Content() = %q", c)
	}
}

func TestParseCSVMissingNameColumn(t *testing.T) {
	if _, err := ParseCSV(strings.NewReader("title,password\nx,y\n")); err == nil {
		t.Fatal("expected error for missing name column")
	}
}

// TestParseDispatch covers the extension router: .csv/.xml route to the right
// parser (case-insensitively) and anything else is a clear error.
func TestParseDispatch(t *testing.T) {
	if _, err := Parse("export.CSV", strings.NewReader("name\nwifi\n")); err != nil {
		t.Errorf(".CSV should route to ParseCSV: %v", err)
	}
	if _, err := Parse("export.XML", strings.NewReader("<store></store>")); err != nil {
		t.Errorf(".XML should route to ParseXML: %v", err)
	}
	if _, err := Parse("export.json", strings.NewReader("{}")); err == nil {
		t.Error("unsupported extension should error")
	}
}

func TestParseXML(t *testing.T) {
	doc := `<store>
		<entry name="aws/key"><password>secret1</password><user>demo</user></entry>
		<entry><name>wifi</name><password>secret2</password></entry>
		<entry name=""><password>orphan</password></entry>
	</store>`

	got, err := ParseXML(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Name: "aws/key", Password: "secret1", Meta: []KV{{"user", "demo"}}},
		{Name: "wifi", Password: "secret2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseXML =\n%+v\nwant\n%+v", got, want)
	}
}
