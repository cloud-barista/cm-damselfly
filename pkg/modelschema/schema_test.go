package modelschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	cloudmodel "github.com/cloud-barista/cm-beetle/imdl/cloud-model"
	onpremisemodel "github.com/cloud-barista/cm-beetle/imdl/on-premise-model"
)

// loadLocalPackage parses the source of the compiled-in package from the module cache (no network access).
func loadLocalPackage(t *testing.T, importPath string) *Package {
	t.Helper()
	bp, err := build.Import(importPath, ".", build.FindOnly)
	if err != nil {
		t.Skipf("cannot locate %s: %v", importPath, err)
	}
	entries, err := os.ReadDir(bp.Dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, e := range entries {
		if isPackageSourceFile(e.Name()) {
			content, err := os.ReadFile(filepath.Join(bp.Dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files[e.Name()] = content
		}
	}
	pkg, err := ParsePackage(files)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

// fill populates every exported field with a non-zero value.
func fill(v reflect.Value, depth int, seed *int) {
	*seed++
	switch v.Kind() {
	case reflect.Pointer:
		if depth > 5 {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), depth+1, seed)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				fill(v.Field(i), depth+1, seed)
			}
		}
	case reflect.Slice:
		if depth > 5 {
			return
		}
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := 0; i < 2; i++ {
			fill(s.Index(i), depth+1, seed)
		}
		v.Set(s)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			fill(v.Index(i), depth+1, seed)
		}
	case reflect.Map:
		if depth > 5 {
			return
		}
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		fill(key, depth+1, seed)
		val := reflect.New(v.Type().Elem()).Elem()
		fill(val, depth+1, seed)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.String:
		v.SetString(fmt.Sprintf("s%d", *seed))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(*seed % 100))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(*seed % 100))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(*seed) + 0.5)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(fmt.Sprintf("any%d", *seed)))
		}
	}
}

// dropEveryOtherKey removes some keys and changes the case of others to exercise defaults and key folding.
func dropEveryOtherKey(v interface{}) interface{} {
	switch vv := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for i, k := range sortedKeys(vv) {
			switch i % 3 {
			case 0:
				continue
			case 1:
				out[strings.ToUpper(k[:1])+k[1:]] = dropEveryOtherKey(vv[k])
			default:
				out[k] = dropEveryOtherKey(vv[k])
			}
		}
		return out
	case []interface{}:
		for i := range vv {
			vv[i] = dropEveryOtherKey(vv[i])
		}
	}
	return v
}

func roundTrip(t *testing.T, input []byte, newTarget func() interface{}) interface{} {
	t.Helper()
	target := newTarget()
	if err := json.Unmarshal(input, target); err != nil {
		t.Fatalf("encoding/json unmarshal: %v", err)
	}
	out, err := json.Marshal(target)
	if err != nil {
		t.Fatal(err)
	}
	var generic interface{}
	if err := json.Unmarshal(out, &generic); err != nil {
		t.Fatal(err)
	}
	return generic
}

func assertEquivalent(t *testing.T, pkg *Package, typeName string, input []byte, newTarget func() interface{}) {
	t.Helper()
	got, err := pkg.Normalize(input, typeName, "model")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	var gotGeneric interface{}
	if err := json.Unmarshal(got, &gotGeneric); err != nil {
		t.Fatal(err)
	}
	want := roundTrip(t, input, newTarget)
	if !reflect.DeepEqual(gotGeneric, want) {
		wantJSON, _ := json.Marshal(want)
		t.Fatalf("normalized output differs from encoding/json round trip\n got: %.2000s\nwant: %.2000s", got, wantJSON)
	}
}

func TestNormalizeMatchesEncodingJSON(t *testing.T) {
	cases := []struct {
		importPath string
		typeName   string
		newTarget  func() interface{}
	}{
		{"github.com/cloud-barista/cm-beetle/imdl/on-premise-model", "OnpremInfra", func() interface{} { return new(onpremisemodel.OnpremInfra) }},
		{"github.com/cloud-barista/cm-beetle/imdl/cloud-model", "RecommendedInfra", func() interface{} { return new(cloudmodel.RecommendedInfra) }},
	}

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			pkg := loadLocalPackage(t, tc.importPath)

			filled := tc.newTarget()
			seed := 0
			fill(reflect.ValueOf(filled).Elem(), 0, &seed)
			full, err := json.Marshal(filled)
			if err != nil {
				t.Fatal(err)
			}

			var generic interface{}
			if err := json.Unmarshal(full, &generic); err != nil {
				t.Fatal(err)
			}
			partial, err := json.Marshal(dropEveryOtherKey(generic))
			if err != nil {
				t.Fatal(err)
			}

			for name, input := range map[string][]byte{"full": full, "partial": partial, "empty": []byte(`{}`), "null": []byte(`null`)} {
				t.Run(name, func(t *testing.T) {
					assertEquivalent(t, pkg, tc.typeName, input, tc.newTarget)
				})
			}
		})
	}
}

const testSource = `package sample

import "time"

type Kind string

type Base struct {
	ID      string ` + "`json:\"id\"`" + `
	Ignored string ` + "`json:\"-\"`" + `
	hidden  string
}

type Extra struct {
	Note string ` + "`json:\"note\"`" + `
}

type Root struct {
	Base
	*Extra
	Name    string            ` + "`json:\"name\"`" + `
	Kind    Kind              ` + "`json:\"kind,omitempty\"`" + `
	Count   int8              ` + "`json:\"count\"`" + `
	Size    uint              ` + "`json:\"size\"`" + `
	Ratio   float64           ` + "`json:\"ratio\"`" + `
	Tags    []string          ` + "`json:\"tags\"`" + `
	Pair    [2]int            ` + "`json:\"pair\"`" + `
	Labels  map[string]string ` + "`json:\"labels,omitempty\"`" + `
	ByPort  map[int]string    ` + "`json:\"byPort\"`" + `
	Created time.Time         ` + "`json:\"created\"`" + `
	Next    *Root             ` + "`json:\"next,omitempty\"`" + `
	Any     interface{}       ` + "`json:\"any\"`" + `
	Data    []byte            ` + "`json:\"data\"`" + `
}
`

type sampleKind string

type SampleBase struct {
	ID      string `json:"id"`
	Ignored string `json:"-"`
	hidden  string
}

type SampleExtra struct {
	Note string `json:"note"`
}

type sampleRoot struct {
	SampleBase
	*SampleExtra
	Name    string            `json:"name"`
	Kind    sampleKind        `json:"kind,omitempty"`
	Count   int8              `json:"count"`
	Size    uint              `json:"size"`
	Ratio   float64           `json:"ratio"`
	Tags    []string          `json:"tags"`
	Pair    [2]int            `json:"pair"`
	Labels  map[string]string `json:"labels,omitempty"`
	ByPort  map[int]string    `json:"byPort"`
	Created time.Time         `json:"created"`
	Next    *sampleRoot       `json:"next,omitempty"`
	Any     interface{}       `json:"any"`
	Data    []byte            `json:"data"`
}

func TestNormalizeSample(t *testing.T) {
	pkg, err := ParsePackage(map[string][]byte{"sample.go": []byte(testSource)})
	if err != nil {
		t.Fatal(err)
	}
	newTarget := func() interface{} { return new(sampleRoot) }

	valid := []string{
		`{}`,
		`{"id":"a","NAME":"n","count":-3,"size":7,"ratio":1.25,"tags":[],"pair":[1],"byPort":{"80":"http"},"created":"2026-01-02T03:04:05+09:00","any":{"x":[1,true]},"data":"aGVsbG8="}`,
		`{"note":"embedded pointer is allocated","next":{"name":"child","pair":[1,2,3]}}`,
		`{"name":null,"tags":null,"labels":{}}`,
	}
	for _, input := range valid {
		assertEquivalent(t, pkg, "Root", []byte(input), newTarget)
	}

	invalid := map[string]string{
		`{"unknown":1}`:             "model.unknown: unknown field",
		`{"hidden":"x"}`:            "model.hidden: unknown field",
		`{"Ignored":"x"}`:           "model.Ignored: unknown field",
		`{"name":1}`:                "model.name: expected string, got number",
		`{"count":300}`:             "model.count: 300 is not a valid int8 value",
		`{"size":-1}`:               "model.size: -1 is not a valid uint value",
		`{"count":1.5}`:             "model.count: 1.5 is not a valid int8 value",
		`{"tags":"a"}`:              "model.tags: expected array, got string",
		`{"byPort":{"http":"x"}}`:   `model.byPort: map key "http" is not a valid int`,
		`{"created":"yesterday"}`:   `model.created: "yesterday" is not a valid RFC 3339 time`,
		`{"next":{"next":{"x":1}}}`: "model.next.next.x: unknown field",
		`{"data":"***"}`:            "model.data: invalid base64 string",
		`[]`:                        "model: expected object, got array",
		`{"name":"a"} {}`:           "unexpected data after the top-level value",
	}
	for input, want := range invalid {
		_, err := pkg.Normalize([]byte(input), "Root", "model")
		var vErr *ValidationError
		if !errors.As(err, &vErr) || !strings.Contains(err.Error(), want) {
			t.Errorf("input %s: got error %v, want it to contain %q", input, err, want)
		}
	}

	if _, err := pkg.Normalize([]byte(`{}`), "Missing", "model"); err == nil {
		t.Error("expected an error for an undefined type")
	}
}
