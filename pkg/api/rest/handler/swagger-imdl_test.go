package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalInTopLevelKeyOrder(t *testing.T) {
	original := `{"swagger":"2.0","info":{"title":"t"},"paths":{"/a":{}},"definitions":{"b":{},"a":{}}}`
	spec := map[string]interface{}{}
	if err := json.Unmarshal([]byte(original), &spec); err != nil {
		t.Fatal(err)
	}
	spec["x-extra"] = true

	out, err := marshalInTopLevelKeyOrder(spec, original)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	want := `{"swagger":"2.0","info":{"title":"t"},"paths":{"/a":{}},"definitions":{"a":{},"b":{}},"x-extra":true}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if strings.Index(got, `"paths"`) > strings.Index(got, `"definitions"`) {
		t.Fatal("'paths' must precede 'definitions'")
	}
}
