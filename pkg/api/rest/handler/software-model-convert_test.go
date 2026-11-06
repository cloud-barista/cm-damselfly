package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	echo "github.com/labstack/echo/v4"
)

// convertTestSmdlSource has the parts of the cm-grasshopper/smdl v0.1.4 struct which the conversion tests need.
const convertTestSmdlSource = `package softwaremodel
type KubernetesResources struct {
	Namespaces []string                  ` + "`json:\"namespaces,omitempty\"`" + `
	Workloads  map[string]map[string]int ` + "`json:\"workloads,omitempty\"`" + `
}
type Binary struct {
	Name             string   ` + "`json:\"name\" validate:\"required\"`" + `
	RequiredPackages []string ` + "`json:\"required_packages\"`" + `
	LaunchType       string   ` + "`json:\"launch_type\"`" + `
	SystemdUnitName  string   ` + "`json:\"systemd_unit_name,omitempty\"`" + `
}
type Kubernetes struct {
	Version    string               ` + "`json:\"version,omitempty\" validate:\"required\"`" + `
	KubeConfig string               ` + "`json:\"kube_config\"`" + `
	Resources  *KubernetesResources ` + "`json:\"resources,omitempty\" validate:\"required\"`" + `
}
type SoftwareList struct {
	Binaries   []Binary     ` + "`json:\"binaries\"`" + `
	Kubernetes []Kubernetes ` + "`json:\"kubernetes\"`" + `
}
type SourceConnectionInfoSoftwareProperty struct {
	ConnectionId string       ` + "`json:\"connection_id\" validate:\"required\"`" + `
	Softwares    SoftwareList ` + "`json:\"softwares\"`" + `
}
type SourceGroupSoftwareProperty struct {
	SourceGroupId      string                                 ` + "`json:\"source_group_id\" validate:\"required\"`" + `
	ConnectionInfoList []SourceConnectionInfoSoftwareProperty ` + "`json:\"connection_info_list\"`" + `
}
type KubernetesMigrationInfo struct {
	Order     int                  ` + "`json:\"order\"`" + `
	Resources *KubernetesResources ` + "`json:\"resources,omitempty\" validate:\"required\"`" + `
}
type MigrationList struct {
	Kubernetes []KubernetesMigrationInfo ` + "`json:\"kubernetes\"`" + `
}
type MigrationServer struct {
	SourceConnectionInfoID string        ` + "`json:\"source_connection_info_id\"`" + `
	MigrationList          MigrationList ` + "`json:\"migration_list\"`" + `
}
type TargetGroupSoftwareProperty struct {
	Servers []MigrationServer ` + "`json:\"servers\"`" + `
}
`

func TestConvertSoftwareModelFillsAddedFields(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestSmdlSource)
	raw := []byte(`{"source_group_id":"g1","connection_info_list":[{"connection_id":"c1","softwares":{"binaries":[{"name":"tomcat"}],"kubernetes":null}}]}`)

	out, conversion, err := convertSoftwareModel(pkg, StageSource, "v0.1.0", "v0.1.4", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !conversion.Converted || conversion.OriginVersion != "v0.1.0" || conversion.CurrentVersion != "v0.1.4" || len(conversion.DroppedFields) != 0 {
		t.Fatalf("unexpected conversion info: %+v", conversion)
	}

	got := decodeJSON(t, out)
	binary := got["connection_info_list"].([]interface{})[0].(map[string]interface{})["softwares"].(map[string]interface{})["binaries"].([]interface{})[0].(map[string]interface{})
	if binary["name"] != "tomcat" {
		t.Fatalf("the stored value was not kept: %v", binary)
	}
	if value, exists := binary["launch_type"]; !exists || value != "" {
		t.Fatalf("'launch_type' was not filled in with the zero value: %v", binary)
	}
	if value, exists := binary["required_packages"]; !exists || value != nil {
		t.Fatalf("'required_packages' was not filled in with the zero value: %v", binary)
	}
	if _, exists := binary["systemd_unit_name"]; exists {
		t.Fatalf("an empty 'omitempty' field was filled in: %v", binary)
	}
}

func TestConvertSoftwareModelDropsUndeclaredResourcesKeys(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestSmdlSource)

	tests := []struct {
		name        string
		stage       ModelStage
		raw         string
		wantDropped string
	}{
		{
			"source model",
			StageSource,
			`{"source_group_id":"g1","connection_info_list":[{"connection_id":"c1","softwares":{"kubernetes":[{"version":"1.32","kube_config":"cfg","resources":{"pods":3,"namespaces":["default"]}}]}}]}`,
			"sourceSoftwareModel.connection_info_list[0].softwares.kubernetes[0].resources.pods",
		},
		{
			"planning model",
			StagePlanning,
			`{"servers":[{"migration_list":{"kubernetes":[{"resources":{"pods":3,"namespaces":["default"]}}]}}]}`,
			"targetSoftwareModel.servers[0].migration_list.kubernetes[0].resources.pods",
		},
		{
			"target model",
			StageTarget,
			`{"servers":[{"migration_list":{"kubernetes":[{"resources":{"pods":3,"namespaces":["default"]}}]}}]}`,
			"targetSoftwareModel.servers[0].migration_list.kubernetes[0].resources.pods",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, conversion, err := convertSoftwareModel(pkg, tt.stage, "v0.1.3", "v0.1.4", []byte(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(conversion.DroppedFields, []string{tt.wantDropped}) {
				t.Fatalf("droppedFields = %v, want [%s]", conversion.DroppedFields, tt.wantDropped)
			}
			if !strings.Contains(string(out), `"namespaces":["default"]`) || strings.Contains(string(out), "pods") {
				t.Fatalf("unexpected resources: %s", out)
			}
		})
	}
}

// A declared key with a value of another type must fail the conversion rather than being removed.
func TestConvertSoftwareModelFailsOnResourcesTypeMismatch(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestSmdlSource)

	for name, resources := range map[string]string{
		"namespaces is not an array":       `{"namespaces":"default"}`,
		"workloads has no namespace level": `{"workloads":{"deployments":3}}`,
		"resources is not an object":       `"all"`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := `{"source_group_id":"g1","connection_info_list":[{"connection_id":"c1","softwares":{"kubernetes":[{"version":"1.32","resources":` + resources + `}]}}]}`
			_, _, err := convertSoftwareModel(pkg, StageSource, "v0.1.3", "v0.1.4", []byte(raw))
			var validationErr *modelschema.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("err = %v, want a *modelschema.ValidationError", err)
			}
		})
	}
}

// A model labeled with an older version can have the content of a newer one, because the legacy APIs
// validated it against the built-in struct.
func TestConvertSoftwareModelKeepsContentNewerThanStoredVersion(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestSmdlSource)
	raw := `{"source_group_id":"g1","connection_info_list":[{"connection_id":"c1","softwares":{"kubernetes":[{"version":"1.32","kube_config":"","resources":{"namespaces":["shop"],"workloads":{"shop":{"Deployment":2}}}}]}}]}`

	out, conversion, err := convertSoftwareModel(pkg, StageSource, "v0.1.3", "v0.1.4", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(conversion.DroppedFields) != 0 {
		t.Fatalf("unexpected dropped fields: %v", conversion.DroppedFields)
	}
	if !strings.Contains(string(out), `"workloads":{"shop":{"Deployment":2}}`) {
		t.Fatalf("the typed resources were not kept: %s", out)
	}
}

func TestValidateSoftwareModelConversion(t *testing.T) {
	versions := []string{"v0.1.0", "v0.1.3", "v0.1.4"}
	tests := []struct {
		name            string
		origin, current string
		wantErr         string
	}{
		{"to a newer version", "v0.1.0", "v0.1.4", ""},
		{"to the same version", "v0.1.4", "v0.1.4", ""},
		{"to an older version", "v0.1.4", "v0.1.3", "only the conversion to a newer version"},
		{"stored version is empty", "", "v0.1.4", "is not a cm-grasshopper/smdl tag"},
		{"stored version is not a tag", "v0.1.0-untagged", "v0.1.4", "is not a cm-grasshopper/smdl tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSoftwareModelConversion(tt.origin, tt.current, versions)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func requestConvertSoftwareModel(id, query string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/software-model/"+id+"/convert?"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := ConvertSoftwareModel(c); err != nil {
		panic(err)
	}
	return rec
}

// The handler tests cover the responses which do not need the source code of a tagged version.
func TestConvertSoftwareModelHandler(t *testing.T) {
	modelVersionCacheMu.Lock()
	previous, hadPrevious := modelVersionCache[softwareModelTagSource.ModulePath]
	modelVersionCache[softwareModelTagSource.ModulePath] = modelVersionCacheEntry{
		versions:  []string{"v0.1.0", "v0.1.3", "v0.1.4"},
		fetchedAt: time.Now(),
	}
	modelVersionCacheMu.Unlock()
	t.Cleanup(func() {
		modelVersionCacheMu.Lock()
		defer modelVersionCacheMu.Unlock()
		if hadPrevious {
			modelVersionCache[softwareModelTagSource.ModulePath] = previous
		} else {
			delete(modelVersionCache, softwareModelTagSource.ModulePath)
		}
	})

	put := func(id string, record map[string]interface{}) {
		if err := lkvstore.Put(id, record); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	body := map[string]interface{}{"source_group_id": "g1"}
	put("sw-convert-latest", map[string]interface{}{"isSoftwareModel": true, "modelStage": "source", "softwareModelVersion": "v0.1.4", "sourceSoftwareModel": body})
	put("sw-convert-old", map[string]interface{}{"isSoftwareModel": true, "modelStage": "source", "softwareModelVersion": "v0.1.3", "sourceSoftwareModel": body})
	put("sw-convert-untagged", map[string]interface{}{"isSoftwareModel": true, "modelStage": "source", "softwareModelVersion": "untagged", "sourceSoftwareModel": body})
	put("sw-convert-noversion", map[string]interface{}{"isSoftwareModel": true, "modelStage": "source", "sourceSoftwareModel": body})
	put("sw-convert-nobody", map[string]interface{}{"isSoftwareModel": true, "modelStage": "target", "softwareModelVersion": "v0.1.0"})
	put("sw-convert-infra", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "onpremModelVersion": "v0.1.19"})
	put("sw-convert-legacy", map[string]interface{}{"isSoftwareModel": true, "isTargetModel": false, "softwareModelVersion": "v0.1.4", "sourceSoftwareModel": body})

	tests := []struct {
		name       string
		id, query  string
		wantStatus int
		wantText   string
	}{
		{"modelStage is required", "sw-convert-old", "", http.StatusBadRequest, "'modelStage' must be"},
		{"model not found", "sw-convert-no-such-model", "modelStage=source", http.StatusNotFound, "failed to find"},
		{"not a software model", "sw-convert-infra", "modelStage=source", http.StatusBadRequest, "is not a software model"},
		{"model stage mismatch", "sw-convert-old", "modelStage=target", http.StatusBadRequest, "model stage mismatch"},
		{"version is not a tag", "sw-convert-old", "modelStage=source&modelVersion=v9.9.9", http.StatusBadRequest, "not a tagged version"},
		{"downgrade", "sw-convert-old", "modelStage=source&modelVersion=v0.1.0", http.StatusBadRequest, "only the conversion to a newer version"},
		{"stored version is not a tag", "sw-convert-untagged", "modelStage=source", http.StatusBadRequest, "is not a cm-grasshopper/smdl tag"},
		{"stored version is empty", "sw-convert-noversion", "modelStage=source", http.StatusBadRequest, "is not a cm-grasshopper/smdl tag"},
		{"stored model has no body", "sw-convert-nobody", "modelStage=target", http.StatusBadRequest, "no 'targetSoftwareModel'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertSoftwareModel(tt.id, tt.query)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tt.wantText) {
				t.Fatalf("body = %s, want it to contain %q", rec.Body, tt.wantText)
			}
		})
	}

	sameVersionCases := []struct {
		name, id, query string
	}{
		{"omitted version means the latest tag", "sw-convert-latest", "modelStage=source"},
		{"explicit same version", "sw-convert-latest", "modelStage=source&modelVersion=v0.1.4"},
		{"legacy record without modelStage", "sw-convert-legacy", "modelStage=source"},
	}
	for _, tt := range sameVersionCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertSoftwareModel(tt.id, tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			got := decodeJSON(t, rec.Body.Bytes())
			conversion := got["conversion"].(map[string]interface{})
			if conversion["converted"] != false || conversion["originVersion"] != "v0.1.4" || conversion["currentVersion"] != "v0.1.4" {
				t.Fatalf("unexpected conversion info: %v", conversion)
			}
			if got["softwareModelVersion"] != "v0.1.4" || got["modelStage"] != "source" {
				t.Fatalf("unexpected model: %v", got)
			}
		})
	}
}

func TestGetSoftwareModelHasNoConversion(t *testing.T) {
	id := "sw-convert-get"
	if err := lkvstore.Put(id, map[string]interface{}{"isSoftwareModel": true, "modelStage": "source", "softwareModelVersion": "v0.1.4"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lkvstore.Delete(id) })

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/software-model/"+id+"?modelStage=source", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := GetSoftwareModel(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if _, exists := decodeJSON(t, rec.Body.Bytes())["conversion"]; exists {
		t.Fatalf("'conversion' must not be in the response of GetSoftwareModel: %s", rec.Body)
	}
}
