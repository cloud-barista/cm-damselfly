package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	echo "github.com/labstack/echo/v4"
)

const convertTestOnpremSource = `package onprem
type OnpremInfra struct {
	Network NetworkProperty ` + "`json:\"network,omitempty\"`" + `
	Nodes   []NodeProperty  ` + "`json:\"nodes\" validate:\"required\"`" + `
}
type NetworkProperty struct {
	Name string ` + "`json:\"name,omitempty\"`" + `
}
type NodeProperty struct {
	Hostname string            ` + "`json:\"hostname\"`" + `
	Role     string            ` + "`json:\"role,omitempty\"`" + `
	DiskGB   uint64            ` + "`json:\"diskGB\"`" + `
	GPUCards []GpuCardProperty ` + "`json:\"gpuCards,omitempty\"`" + `
}
type GpuCardProperty struct {
	DriverIndex   string  ` + "`json:\"driverIndex,omitempty\"`" + `
	Vendor        string  ` + "`json:\"vendor,omitempty\"`" + `
	Model         string  ` + "`json:\"model,omitempty\"`" + `
	MemoryTotalGB float32 ` + "`json:\"memoryTotalGB,omitempty\"`" + `
}
`

const convertTestCloudSource = `package cloud
type RecommendedInfra struct {
	TargetInfra       InfraReq  ` + "`json:\"targetInfra\"`" + `
	TargetOsImageList []OsImage ` + "`json:\"targetOsImageList\"`" + `
	TargetSpecList    []Spec    ` + "`json:\"targetSpecList\"`" + `
}
type InfraReq struct {
	Name         string           ` + "`json:\"name\"`" + `
	NodeGroups   []NodeGroup      ` + "`json:\"nodeGroups\"`" + `
	PostCommands []PostCommandReq ` + "`json:\"postCommands,omitempty\"`" + `
}
type NodeGroup struct {
	NodeGroupSize    int    ` + "`json:\"nodeGroupSize\"`" + `
	NodeUserName     string ` + "`json:\"nodeUserName,omitempty\"`" + `
	NodeUserPassword string ` + "`json:\"nodeUserPassword,omitempty\"`" + `
}
type PostCommandReq struct {
	Command  []string ` + "`json:\"command\"`" + `
	UserName string   ` + "`json:\"userName\"`" + `
}
type OsImage struct {
	SourceNodeUid string ` + "`json:\"sourceNodeUid\"`" + `
}
type Spec struct {
	Id string ` + "`json:\"id\"`" + `
}
`

func parseConvertTestPackage(t *testing.T, source string) *modelschema.Package {
	t.Helper()
	pkg, err := modelschema.ParsePackage(map[string][]byte{"model.go": []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func decodeJSON(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON %s: %v", data, err)
	}
	return out
}

func TestConvertOnpremServersToNodes(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestOnpremSource)
	raw := `{"network":{"name":"n"},"servers":[{"hostname":"h1","diskGB":1024}]}`

	out, conversion, err := convertInfraModel(pkg, false, "v0.1.0", "v0.1.19", json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}

	got := decodeJSON(t, out)
	nodes, ok := got["nodes"].([]interface{})
	if !ok || len(nodes) != 1 {
		t.Fatalf("'servers' must be moved to 'nodes': %s", out)
	}
	if _, exists := got["servers"]; exists {
		t.Fatalf("'servers' must not remain: %s", out)
	}
	node := nodes[0].(map[string]interface{})
	if node["hostname"] != "h1" {
		t.Fatalf("the node values must be kept: %s", out)
	}
	if node["diskGB"] != float64(1024) {
		t.Fatalf("the disk size must not be changed: %s", out)
	}
	if !conversion.Converted || conversion.OriginVersion != "v0.1.0" || conversion.CurrentVersion != "v0.1.19" {
		t.Fatalf("unexpected conversion info: %+v", conversion)
	}
	if len(conversion.DroppedFields) != 0 || len(conversion.Warnings) != 0 {
		t.Fatalf("nothing must be lost: %+v", conversion)
	}
}

func TestMigrationsApplyOnlyToVersionsInRange(t *testing.T) {
	raw := json.RawMessage(`{"servers":[]}`)
	tests := []struct {
		name            string
		origin, current string
		wantNodes       bool
	}{
		{"origin is older than the change, current includes it", "v0.1.3", "v0.1.4", true},
		{"current is older than the change", "v0.1.0", "v0.1.3", false},
		{"origin already includes the change", "v0.1.4", "v0.1.19", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := migrateInfraModel(onpremInfraModelMigrations, tt.origin, tt.current, raw)
			if err != nil {
				t.Fatal(err)
			}
			if _, hasNodes := decodeJSON(t, out)["nodes"]; hasNodes != tt.wantNodes {
				t.Fatalf("hasNodes = %v, want %v: %s", hasNodes, tt.wantNodes, out)
			}
		})
	}
}

func TestConvertOnpremGpuToGpuCards(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestOnpremSource)

	t.Run("one card per detail", func(t *testing.T) {
		raw := `{"nodes":[{"hostname":"h","gpu":{"count":2,"vendor":"NVIDIA","model":"primary","totalMemoryGB":80,
			"details":[{"index":0,"model":"m0","memoryTotal":40},{"index":1,"memoryTotal":40}]}}]}`
		out, conversion, err := convertInfraModel(pkg, false, "v0.1.14", "v0.1.15", json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		node := decodeJSON(t, out)["nodes"].([]interface{})[0].(map[string]interface{})
		if _, exists := node["gpu"]; exists {
			t.Fatalf("'gpu' must not remain: %s", out)
		}
		cards := node["gpuCards"].([]interface{})
		if len(cards) != 2 {
			t.Fatalf("want 2 cards: %s", out)
		}
		first, second := cards[0].(map[string]interface{}), cards[1].(map[string]interface{})
		if first["driverIndex"] != "0" || first["model"] != "m0" || first["vendor"] != "NVIDIA" || first["memoryTotalGB"] != float64(40) {
			t.Fatalf("unexpected first card: %v", first)
		}
		if second["driverIndex"] != "1" || second["model"] != "primary" {
			t.Fatalf("the card without its own model must have the common one: %v", second)
		}
		if len(conversion.Warnings) != 0 || len(conversion.DroppedFields) != 0 {
			t.Fatalf("nothing must be lost: %+v", conversion)
		}
	})

	t.Run("only count generates cards with a warning", func(t *testing.T) {
		raw := `{"nodes":[{"hostname":"h","gpu":{"count":2,"vendor":"NVIDIA","totalMemoryGB":80}}]}`
		out, conversion, err := convertInfraModel(pkg, false, "v0.1.14", "v0.1.19", json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		cards := decodeJSON(t, out)["nodes"].([]interface{})[0].(map[string]interface{})["gpuCards"].([]interface{})
		if len(cards) != 2 {
			t.Fatalf("want 2 cards: %s", out)
		}
		if _, exists := cards[0].(map[string]interface{})["memoryTotalGB"]; exists {
			t.Fatalf("the total memory must not be divided: %s", out)
		}
		if len(conversion.Warnings) != 2 {
			t.Fatalf("want warnings about the generated cards and 'totalMemoryGB': %v", conversion.Warnings)
		}
	})

	t.Run("excessive count generates no cards", func(t *testing.T) {
		raw := `{"nodes":[{"hostname":"h","gpu":{"count":4294967295}}]}`
		out, conversion, err := convertInfraModel(pkg, false, "v0.1.14", "v0.1.19", json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := decodeJSON(t, out)["nodes"].([]interface{})[0].(map[string]interface{})["gpuCards"]; exists {
			t.Fatalf("no cards must be generated: %s", out)
		}
		if len(conversion.Warnings) != 1 {
			t.Fatalf("want a warning: %v", conversion.Warnings)
		}
	})

	t.Run("mismatched count is warned", func(t *testing.T) {
		raw := `{"nodes":[{"hostname":"h","gpu":{"count":3,"details":[{"index":0}]}}]}`
		_, conversion, err := convertInfraModel(pkg, false, "v0.1.14", "v0.1.19", json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if len(conversion.Warnings) != 1 || !strings.Contains(conversion.Warnings[0], "'count'") {
			t.Fatalf("want a warning about 'count': %v", conversion.Warnings)
		}
	})
}

func TestConvertCloudLegacyNames(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestCloudSource)
	raw := `{
		"targetVmInfra":{"name":"i","subGroups":[{"subGroupSize":2,"vmUserName":"u","vmUserPassword":"p"}],
			"postCommand":{"command":["ls"],"userName":"u"}},
		"targetVmOsImageList":[{"sourceVmUid":"s"}],
		"targetVmSpecList":[{"id":"spec"}]
	}`

	out, conversion, err := convertInfraModel(pkg, true, "v0.1.0", "v0.1.19", json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(conversion.DroppedFields) != 0 {
		t.Fatalf("every field has a new name, so nothing must be dropped: %v", conversion.DroppedFields)
	}

	got := decodeJSON(t, out)
	targetInfra := got["targetInfra"].(map[string]interface{})
	group := targetInfra["nodeGroups"].([]interface{})[0].(map[string]interface{})
	if group["nodeGroupSize"] != float64(2) || group["nodeUserName"] != "u" || group["nodeUserPassword"] != "p" {
		t.Fatalf("unexpected node group: %s", out)
	}
	if len(targetInfra["postCommands"].([]interface{})) != 1 {
		t.Fatalf("'postCommand' must become an entry of 'postCommands': %s", out)
	}
	if got["targetOsImageList"].([]interface{})[0].(map[string]interface{})["sourceNodeUid"] != "s" {
		t.Fatalf("'sourceVmUid' must be moved: %s", out)
	}
	if got["targetSpecList"].([]interface{})[0].(map[string]interface{})["id"] != "spec" {
		t.Fatalf("'targetVmSpecList' must be moved: %s", out)
	}
}

func TestConvertCloudEmptyPostCommandIsNotAPhase(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestCloudSource)
	for _, raw := range []string{
		`{"targetInfra":{"postCommand":{"command":null,"userName":""}}}`,
		`{"targetInfra":{"postCommand":{"command":[""],"userName":"u"}}}`,
	} {
		out, conversion, err := convertInfraModel(pkg, true, "v0.1.6", "v0.1.19", json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := decodeJSON(t, out)["targetInfra"].(map[string]interface{})["postCommands"]; exists {
			t.Fatalf("an empty 'postCommand' must not become a phase: %s", out)
		}
		if len(conversion.DroppedFields) != 0 {
			t.Fatalf("unexpected dropped fields: %v", conversion.DroppedFields)
		}
	}
}

func TestConvertDropsFieldsOfRemovedVersions(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestCloudSource)
	out, conversion, err := convertInfraModel(pkg, true, "v0.1.3", "v0.1.19", json.RawMessage(`{"nameSeed":"x","targetInfra":{"name":"i"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := decodeJSON(t, out)["nameSeed"]; exists {
		t.Fatalf("'nameSeed' must be removed: %s", out)
	}
	if len(conversion.DroppedFields) != 1 || conversion.DroppedFields[0] != "cloudInfraModel.nameSeed" {
		t.Fatalf("unexpected dropped fields: %v", conversion.DroppedFields)
	}
}

// A stored model may be newer than its version, like one with 'nodes' which is stored as v0.1.0.
func TestConvertKeepsContentNewerThanStoredVersion(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestOnpremSource)
	out, conversion, err := convertInfraModel(pkg, false, "v0.1.0", "v0.1.19", json.RawMessage(`{"nodes":[{"hostname":"h","role":"worker"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	node := decodeJSON(t, out)["nodes"].([]interface{})[0].(map[string]interface{})
	if node["hostname"] != "h" || node["role"] != "worker" {
		t.Fatalf("the newer content must be kept: %s", out)
	}
	if len(conversion.DroppedFields) != 0 {
		t.Fatalf("unexpected dropped fields: %v", conversion.DroppedFields)
	}
}

func TestConvertReportsOldFieldWhenNewNameIsUsed(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestOnpremSource)
	out, conversion, err := convertInfraModel(pkg, false, "v0.1.0", "v0.1.19", json.RawMessage(`{"servers":[{"hostname":"old"}],"nodes":[{"hostname":"new"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeJSON(t, out)["nodes"].([]interface{})[0].(map[string]interface{})["hostname"]; got != "new" {
		t.Fatalf("the existing 'nodes' must be kept: %s", out)
	}
	if len(conversion.DroppedFields) != 1 || conversion.DroppedFields[0] != "onpremiseInfraModel.servers" {
		t.Fatalf("the unmoved 'servers' must be reported: %v", conversion.DroppedFields)
	}
}

func TestConvertFailsOnTypeMismatch(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestOnpremSource)
	_, _, err := convertInfraModel(pkg, false, "v0.1.4", "v0.1.19", json.RawMessage(`{"nodes":"not an array"}`))
	if err == nil {
		t.Fatal("a type mismatch must fail")
	}
}

func TestValidateInfraModelConversion(t *testing.T) {
	versions := []string{"v0.1.0", "v0.1.1", "v0.1.2", "v0.1.9", "v0.1.10", "v0.1.19"}
	tests := []struct {
		name            string
		isCloud         bool
		origin, current string
		wantErr         string
	}{
		{"onprem upgrade", false, "v0.1.0", "v0.1.19", ""},
		{"same version", false, "v0.1.9", "v0.1.9", ""},
		{"versions are compared as numbers", false, "v0.1.9", "v0.1.10", ""},
		{"onprem can target v0.1.0", false, "v0.1.0", "v0.1.0", ""},
		{"cloud upgrade to v0.1.2", true, "v0.1.0", "v0.1.2", ""},
		{"downgrade", false, "v0.1.10", "v0.1.9", "only the conversion to a newer version"},
		{"cloud v0.1.0 to v0.1.1", true, "v0.1.0", "v0.1.1", "does not exist before"},
		{"cloud to v0.1.0", true, "v0.1.0", "v0.1.0", "does not exist before"},
		{"stored version is not a tag", false, "v9.9.9", "v0.1.19", "not a cm-beetle/imdl tag"},
		{"stored version is empty", false, "", "v0.1.19", "not a cm-beetle/imdl tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInfraModelConversion(tt.isCloud, tt.origin, tt.current, versions)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func requestConvertInfraModel(id, query string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/infra-model/"+id+"/convert?"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := ConvertInfraModel(c); err != nil {
		panic(err)
	}
	return rec
}

// The handler tests cover the responses which do not need the source code of a tagged version.
func TestConvertInfraModelHandler(t *testing.T) {
	modelVersionCacheMu.Lock()
	previous, hadPrevious := modelVersionCache[infraModelTagSource.ModulePath]
	modelVersionCache[infraModelTagSource.ModulePath] = modelVersionCacheEntry{
		versions:  []string{"v0.1.0", "v0.1.1", "v0.1.2", "v0.1.18", "v0.1.19"},
		fetchedAt: time.Now(),
	}
	modelVersionCacheMu.Unlock()
	t.Cleanup(func() {
		modelVersionCacheMu.Lock()
		defer modelVersionCacheMu.Unlock()
		if hadPrevious {
			modelVersionCache[infraModelTagSource.ModulePath] = previous
		} else {
			delete(modelVersionCache, infraModelTagSource.ModulePath)
		}
	})

	put := func(id string, record map[string]interface{}) {
		if err := lkvstore.Put(id, record); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	put("convert-onprem-latest", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "onpremModelVersion": "v0.1.19", "onpremiseInfraModel": map[string]interface{}{"nodes": []interface{}{}}})
	put("convert-onprem-new", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "onpremModelVersion": "v0.1.18", "onpremiseInfraModel": map[string]interface{}{"nodes": []interface{}{}}})
	put("convert-onprem-untagged", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "onpremModelVersion": "untagged", "onpremiseInfraModel": map[string]interface{}{}})
	put("convert-onprem-nobody", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "onpremModelVersion": "v0.1.0"})
	put("convert-cloud-old", map[string]interface{}{"isCloudModel": true, "modelStage": "target", "cloudModelVersion": "v0.1.0", "cloudInfraModel": map[string]interface{}{}})

	tests := []struct {
		name       string
		id, query  string
		wantStatus int
		wantText   string
	}{
		{"modelType is required", "convert-onprem-new", "", http.StatusBadRequest, "'modelType' must be"},
		{"model not found", "convert-no-such-model", "modelType=onprem", http.StatusNotFound, "failed to find"},
		{"model type mismatch", "convert-onprem-new", "modelType=cloud", http.StatusBadRequest, "model type mismatch"},
		{"version is not a tag", "convert-onprem-new", "modelType=onprem&modelVersion=v9.9.9", http.StatusBadRequest, "not a tagged version"},
		{"downgrade", "convert-onprem-new", "modelType=onprem&modelVersion=v0.1.2", http.StatusBadRequest, "only the conversion to a newer version"},
		{"stored version is not a tag", "convert-onprem-untagged", "modelType=onprem", http.StatusBadRequest, "not a cm-beetle/imdl tag"},
		{"cloud v0.1.0 to v0.1.1", "convert-cloud-old", "modelType=cloud&modelVersion=v0.1.1", http.StatusBadRequest, "does not exist before"},
		{"stored model has no body", "convert-onprem-nobody", "modelType=onprem", http.StatusBadRequest, "no 'onpremiseInfraModel'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertInfraModel(tt.id, tt.query)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tt.wantText) {
				t.Fatalf("body = %s, want it to contain %q", rec.Body, tt.wantText)
			}
		})
	}

	sameVersionCases := []struct {
		name, query string
	}{
		{"omitted version means the latest tag", "modelType=onprem"},
		{"explicit same version", "modelType=onprem&modelVersion=v0.1.19"},
	}
	for _, tt := range sameVersionCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertInfraModel("convert-onprem-latest", tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			got := decodeJSON(t, rec.Body.Bytes())
			conversion := got["conversion"].(map[string]interface{})
			if conversion["converted"] != false || conversion["originVersion"] != "v0.1.19" || conversion["currentVersion"] != "v0.1.19" {
				t.Fatalf("unexpected conversion info: %v", conversion)
			}
			if got["onpremModelVersion"] != "v0.1.19" {
				t.Fatalf("unexpected model version: %v", got["onpremModelVersion"])
			}
		})
	}
}

func TestGetInfraModelHasNoConversion(t *testing.T) {
	if err := lkvstore.Put("convert-get", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "onpremModelVersion": "v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lkvstore.Delete("convert-get") })

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/infra-model/convert-get?modelType=onprem", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("convert-get")
	if err := GetInfraModel(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	got := decodeJSON(t, rec.Body.Bytes())
	if _, exists := got["conversion"]; exists {
		t.Fatalf("GetInfraModel must not have 'conversion': %s", rec.Body)
	}
	if got["onpremModelVersion"] != "v0.1.0" {
		t.Fatalf("unexpected model version: %s", rec.Body)
	}
}
