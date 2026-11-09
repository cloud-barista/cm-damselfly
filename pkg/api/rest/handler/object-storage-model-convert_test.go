package handler

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	echo "github.com/labstack/echo/v4"
)

// convertTestObjectStorageSource has the 'storage-model' structs of cm-beetle/imdl v0.1.20 (the same since v0.1.11).
const convertTestObjectStorageSource = `package storagemodel
type RecommendedObjectStorage struct {
	Status               string                ` + "`json:\"status\"`" + `
	Description          string                ` + "`json:\"description\"`" + `
	Warnings             []string              ` + "`json:\"warnings,omitempty\"`" + `
	TargetCloud          CloudProperty         ` + "`json:\"targetCloud\"`" + `
	TargetObjectStorages []TargetObjectStorage ` + "`json:\"targetObjectStorages\"`" + `
}
type TargetObjectStorage struct {
	SourceBucketName string ` + "`json:\"sourceBucketName\"`" + `
	BucketName       string ` + "`json:\"bucketName\"`" + `
	BucketSpecProperty
}
type SourceObjectStorage struct {
	Description          string                        ` + "`json:\"description,omitempty\"`" + `
	SourceCloud          *CloudProperty                ` + "`json:\"sourceCloud,omitempty\"`" + `
	SourceObjectStorages []SourceObjectStorageProperty ` + "`json:\"sourceObjectStorages\" validate:\"required,min=1\"`" + `
}
type SourceObjectStorageProperty struct {
	BucketName string ` + "`json:\"bucketName\" validate:\"required\"`" + `
	BucketFeatureProperty
	BucketUsageProperty
	BucketMetaProperty
}
type CloudProperty struct {
	Csp    string ` + "`json:\"csp\"`" + `
	Region string ` + "`json:\"region\"`" + `
}
type CORSRule struct {
	AllowedOrigin []string ` + "`json:\"allowedOrigin\" validate:\"required\"`" + `
	AllowedMethod []string ` + "`json:\"allowedMethod\" validate:\"required\"`" + `
	AllowedHeader []string ` + "`json:\"allowedHeader,omitempty\"`" + `
	ExposeHeader  []string ` + "`json:\"exposeHeader,omitempty\"`" + `
	MaxAgeSeconds int      ` + "`json:\"maxAgeSeconds,omitempty\"`" + `
}
type BucketSpecProperty struct {
	VersioningEnabled bool       ` + "`json:\"versioningEnabled\"`" + `
	CORSEnabled       bool       ` + "`json:\"corsEnabled\"`" + `
	CORSRule          []CORSRule ` + "`json:\"corsRule,omitempty\"`" + `
}
type BucketFeatureProperty struct {
	VersioningEnabled bool       ` + "`json:\"versioningEnabled,omitempty\"`" + `
	CORSEnabled       bool       ` + "`json:\"corsEnabled,omitempty\"`" + `
	CORSRule          []CORSRule ` + "`json:\"corsRule,omitempty\"`" + `
	EncryptionEnabled bool       ` + "`json:\"encryptionEnabled,omitempty\"`" + `
	IsPublic          bool       ` + "`json:\"isPublic,omitempty\"`" + `
}
type BucketUsageProperty struct {
	TotalSizeBytes  int64  ` + "`json:\"totalSizeBytes,omitempty\"`" + `
	ObjectCount     int64  ` + "`json:\"objectCount,omitempty\"`" + `
	AccessFrequency string ` + "`json:\"accessFrequency,omitempty\"`" + `
}
type BucketMetaProperty struct {
	Tags         map[string]string ` + "`json:\"tags,omitempty\"`" + `
	CreationDate string            ` + "`json:\"creationDate,omitempty\"`" + `
}
`

// convertTestSourceObjectStorageV0110 is a source object storage model in the form of cm-beetle/imdl v0.1.3 ~ v0.1.10 (a single bucket).
const convertTestSourceObjectStorageV0110 = `{
	"bucketName": "legacy-bucket",
	"versioningEnabled": true,
	"corsEnabled": true,
	"corsRule": [{"allowedOrigin": ["*"], "allowedMethod": ["GET", "PUT"], "maxAgeSeconds": 3000}],
	"encryptionEnabled": true,
	"totalSizeBytes": 1073741824,
	"objectCount": 1200,
	"accessFrequency": "frequent",
	"tags": {"env": "prod"},
	"creationDate": "2025-09-04T04:18:06Z"
}`

func TestConvertSourceObjectStorageModelFromV0110(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestObjectStorageSource)

	for _, stage := range []ModelStage{StageSource, StagePlanning} {
		t.Run(string(stage), func(t *testing.T) {
			out, conversion, err := convertObjectStorageModel(pkg, stage, "v0.1.10", "v0.1.20", []byte(convertTestSourceObjectStorageV0110))
			if err != nil {
				t.Fatal(err)
			}
			if !conversion.Converted || conversion.OriginVersion != "v0.1.10" || conversion.CurrentVersion != "v0.1.20" {
				t.Fatalf("unexpected conversion info: %+v", conversion)
			}
			if len(conversion.DroppedFields) != 0 {
				t.Fatalf("droppedFields = %v, want none", conversion.DroppedFields)
			}
			if len(conversion.Warnings) != 1 || !strings.Contains(conversion.Warnings[0], "'sourceObjectStorageModel.sourceObjectStorages[0]'") {
				t.Fatalf("warnings = %v, want one about the conversion to 'sourceObjectStorages[0]'", conversion.Warnings)
			}

			got := decodeJSON(t, out)
			if buckets, _ := got["sourceObjectStorages"].([]interface{}); len(buckets) != 1 {
				t.Fatalf("sourceObjectStorages = %v, want 1 bucket", got["sourceObjectStorages"])
			}
			for _, name := range sourceBucketFieldsUpToV0110 {
				if _, exists := got[name]; exists {
					t.Fatalf("'%s' was left at the top level: %v", name, got)
				}
			}
			bucket := pathOf(t, got, "sourceObjectStorages", 0)
			expected := map[string]interface{}{
				"bucketName":        "legacy-bucket",
				"versioningEnabled": true,
				"corsEnabled":       true,
				"corsRule": []interface{}{map[string]interface{}{
					"allowedOrigin": []interface{}{"*"}, "allowedMethod": []interface{}{"GET", "PUT"}, "maxAgeSeconds": float64(3000),
				}},
				"encryptionEnabled": true,
				"totalSizeBytes":    float64(1073741824),
				"objectCount":       float64(1200),
				"accessFrequency":   "frequent",
				"tags":              map[string]interface{}{"env": "prod"},
				"creationDate":      "2025-09-04T04:18:06Z",
			}
			if !reflect.DeepEqual(bucket, expected) {
				t.Fatalf("converted bucket:\n got  %v\n want %v", bucket, expected)
			}
		})
	}
}

// The migration of v0.1.11 applies only if the stored model is older than v0.1.11.
func TestObjectStorageModelMigrationsApplyOnlyToVersionsInRange(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestObjectStorageSource)
	raw := `{"sourceObjectStorages":[{"bucketName":"b1"}],"bucketName":"stray","objectCount":3}`

	out, conversion, err := convertObjectStorageModel(pkg, StageSource, "v0.1.11", "v0.1.20", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sourceObjectStorageModel.bucketName", "sourceObjectStorageModel.objectCount"}
	dropped := append([]string(nil), conversion.DroppedFields...)
	sort.Strings(dropped)
	if !reflect.DeepEqual(dropped, want) || len(conversion.Warnings) != 0 {
		t.Fatalf("droppedFields = %v, warnings = %v, want the old fields dropped without migration", conversion.DroppedFields, conversion.Warnings)
	}
	if name := pathOf(t, decodeJSON(t, out), "sourceObjectStorages", 0, "bucketName"); name != "b1" {
		t.Fatalf("the stored bucket was not kept: %s", out)
	}
}

// A model labeled with an older version can already have the content of a newer one; it must be kept as it is,
// and the fields of the single bucket beside it are reported as dropped fields.
func TestConvertObjectStorageModelKeepsContentNewerThanStoredVersion(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestObjectStorageSource)
	raw := `{"description":"group","sourceCloud":{"csp":"aws","region":"ap-northeast-2"},"sourceObjectStorages":[{"bucketName":"b1","isPublic":true},{"bucketName":"b2"}],"bucketName":"stray"}`

	out, conversion, err := convertObjectStorageModel(pkg, StageSource, "v0.1.5", "v0.1.20", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"sourceObjectStorageModel.bucketName"}; !reflect.DeepEqual(conversion.DroppedFields, want) || len(conversion.Warnings) != 0 {
		t.Fatalf("unexpected conversion info: %+v", conversion)
	}
	got := decodeJSON(t, out)
	if buckets, _ := got["sourceObjectStorages"].([]interface{}); len(buckets) != 2 {
		t.Fatalf("the stored buckets were not kept: %v", got)
	}
	if region := pathOf(t, got, "sourceCloud", "region"); region != "ap-northeast-2" {
		t.Fatalf("the stored sourceCloud was not kept: %v", got)
	}
}

// A v0.1.10 model without any bucket field is not wrapped into an empty bucket, and no migration is reported.
// (The 'required' tags are not enforced by modelschema, as with the other models.)
func TestConvertSourceObjectStorageModelWithoutBucket(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestObjectStorageSource)
	out, conversion, err := convertObjectStorageModel(pkg, StageSource, "v0.1.10", "v0.1.20", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(conversion.Warnings) != 0 || len(conversion.DroppedFields) != 0 {
		t.Fatalf("unexpected conversion info: %+v", conversion)
	}
	if buckets := decodeJSON(t, out)["sourceObjectStorages"]; buckets != nil {
		t.Fatalf("sourceObjectStorages = %v, want none", buckets)
	}
}

func TestConvertTargetObjectStorageModelNameSeed(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestObjectStorageSource)
	const target = `"status":"recommended","description":"d","targetCloud":{"csp":"aws","region":"ap-northeast-2"},"targetObjectStorages":[{"sourceBucketName":"src","bucketName":"mig-bucket-01","versioningEnabled":true,"corsEnabled":false}]`

	tests := []struct {
		name        string
		raw         string
		origin      string
		wantWarning bool
	}{
		{"nameSeed with a value", `{"nameSeed":"mig01",` + target + `}`, "v0.1.5", true},
		{"empty nameSeed", `{"nameSeed":"",` + target + `}`, "v0.1.5", false},
		// A v0.1.6 model cannot have 'nameSeed', so it is only dropped, without the migration warning.
		{"v0.1.6 or later", `{"nameSeed":"mig01",` + target + `}`, "v0.1.6", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, conversion, err := convertObjectStorageModel(pkg, StageTarget, tt.origin, "v0.1.20", []byte(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{"targetObjectStorageModel.nameSeed"}; !reflect.DeepEqual(conversion.DroppedFields, want) {
				t.Fatalf("droppedFields = %v, want %v", conversion.DroppedFields, want)
			}
			hasWarning := len(conversion.Warnings) == 1 && strings.Contains(conversion.Warnings[0], "'targetObjectStorageModel.nameSeed' (mig01)")
			if hasWarning != tt.wantWarning || (!tt.wantWarning && len(conversion.Warnings) != 0) {
				t.Fatalf("warnings = %v, want a nameSeed warning: %t", conversion.Warnings, tt.wantWarning)
			}
			got := decodeJSON(t, out)
			if _, exists := got["nameSeed"]; exists {
				t.Fatalf("nameSeed was not dropped: %v", got)
			}
			if name := pathOf(t, got, "targetObjectStorages", 0, "bucketName"); name != "mig-bucket-01" {
				t.Fatalf("the target bucket was not kept: %v", got)
			}
		})
	}
}

func TestValidateObjectStorageModelConversion(t *testing.T) {
	versions := []string{"v0.1.3", "v0.1.10", "v0.1.11", "v0.1.19", "v0.1.20"}
	tests := []struct {
		name            string
		origin, current string
		wantErr         string
	}{
		{"from the oldest to the latest version", "v0.1.3", "v0.1.20", ""},
		{"to a newer version", "v0.1.10", "v0.1.11", ""},
		{"to the same version", "v0.1.20", "v0.1.20", ""},
		{"to an older version", "v0.1.20", "v0.1.19", "only the conversion to a newer version"},
		{"to an older version across the restructure", "v0.1.11", "v0.1.10", "only the conversion to a newer version"},
		{"stored version without the object storage model", "v0.1.2", "v0.1.20", "is not a cm-beetle/imdl tag with the object storage model"},
		{"stored version is empty", "", "v0.1.20", "is not a cm-beetle/imdl tag with the object storage model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateObjectStorageModelConversion(tt.origin, tt.current, versions)
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

func TestFilterObjectStorageModelVersions(t *testing.T) {
	got := filterObjectStorageModelVersions([]string{"v0.1.0", "v0.1.2", "v0.1.3", "v0.1.10", "v0.1.20"})
	if want := []string{"v0.1.3", "v0.1.10", "v0.1.20"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := filterObjectStorageModelVersions([]string{"v0.1.2"}); got == nil || len(got) != 0 {
		t.Fatalf("got %v, want an empty list", got)
	}
}

func TestStoredModelCategoryObjectStorage(t *testing.T) {
	if c, ok := storedModelCategory(map[string]interface{}{"isObjectStorageModel": true}); !ok || c != categoryObjectStorage {
		t.Fatalf("got (%s, %t), want (%s, true)", c, ok, categoryObjectStorage)
	}
	if _, ok := storedModelCategory(map[string]interface{}{"isObjectStorageModel": false}); ok {
		t.Fatal("a model with 'isObjectStorageModel: false' and no other category must have no category")
	}
}

func TestParseModelCategory(t *testing.T) {
	for name, want := range map[string]modelCategory{
		"infra": categoryInfra, "SOFTWARE": categorySoftware, "rdbms": categoryRDBMS,
		"objectStorage": categoryObjectStorage, "objectstorage": categoryObjectStorage,
	} {
		if got, ok := parseModelCategory(name); !ok || got != want {
			t.Fatalf("parseModelCategory(%q) = (%s, %t), want (%s, true)", name, got, ok, want)
		}
	}
	if _, ok := parseModelCategory("blockStorage"); ok {
		t.Fatal("an unknown category must not be parsed")
	}
}

func requestConvertObjectStorageModel(id, query string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/object-storage-model/"+id+"/convert?"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := ConvertObjectStorageModel(c); err != nil {
		panic(err)
	}
	return rec
}

// The handler tests cover the responses which do not need the source code of a tagged version.
func TestConvertStorageModelHandler(t *testing.T) {
	setTestInfraModelVersions(t, []string{"v0.1.2", "v0.1.3", "v0.1.19", "v0.1.20"})

	put := func(id string, record map[string]interface{}) {
		if err := lkvstore.Put(id, record); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	body := map[string]interface{}{"sourceObjectStorages": []interface{}{}}
	put("os-convert-latest", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "source", "objectStorageModelVersion": "v0.1.20", "sourceObjectStorageModel": body})
	put("os-convert-old", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "planning", "objectStorageModelVersion": "v0.1.19", "sourceObjectStorageModel": body})
	put("os-convert-pre-storage", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "source", "objectStorageModelVersion": "v0.1.2", "sourceObjectStorageModel": body})
	put("os-convert-noversion", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "source", "sourceObjectStorageModel": body})
	put("os-convert-nobody", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "target", "objectStorageModelVersion": "v0.1.3"})
	put("os-convert-rdbms", map[string]interface{}{"isRDBMSModel": true, "modelStage": "source", "rdbmsModelVersion": "v0.1.20"})

	tests := []struct {
		name       string
		id, query  string
		wantStatus int
		wantText   string
	}{
		{"modelStage is required", "os-convert-old", "", http.StatusBadRequest, "'modelStage' must be"},
		{"model not found", "os-convert-no-such-model", "modelStage=source", http.StatusNotFound, "failed to find"},
		{"not an object storage model", "os-convert-rdbms", "modelStage=source", http.StatusBadRequest, "is not an object storage model"},
		{"model stage mismatch", "os-convert-old", "modelStage=source", http.StatusBadRequest, "model stage mismatch"},
		{"version is not a tag", "os-convert-old", "modelStage=planning&objectStorageModelVersion=v9.9.9", http.StatusBadRequest, "not a tagged version"},
		{"version without the object storage model", "os-convert-old", "modelStage=planning&objectStorageModelVersion=v0.1.2", http.StatusBadRequest, "not a tagged version"},
		{"downgrade", "os-convert-old", "modelStage=planning&objectStorageModelVersion=v0.1.3", http.StatusBadRequest, "only the conversion to a newer version"},
		{"stored version without the object storage model", "os-convert-pre-storage", "modelStage=source", http.StatusBadRequest, "is not a cm-beetle/imdl tag with the object storage model"},
		{"stored version is empty", "os-convert-noversion", "modelStage=source", http.StatusBadRequest, "is not a cm-beetle/imdl tag with the object storage model"},
		{"stored model has no body", "os-convert-nobody", "modelStage=target", http.StatusBadRequest, "no 'targetObjectStorageModel'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertObjectStorageModel(tt.id, tt.query)
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
		{"omitted version means the latest tag (v0.1.20)", "modelStage=source"},
		{"explicit same version", "modelStage=source&objectStorageModelVersion=v0.1.20"},
	}
	for _, tt := range sameVersionCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertObjectStorageModel("os-convert-latest", tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			got := decodeJSON(t, rec.Body.Bytes())
			conversion := got["conversion"].(map[string]interface{})
			if conversion["converted"] != false || conversion["originVersion"] != "v0.1.20" || conversion["currentVersion"] != "v0.1.20" {
				t.Fatalf("unexpected conversion info: %v", conversion)
			}
			if got["objectStorageModelVersion"] != "v0.1.20" || got["modelStage"] != "source" {
				t.Fatalf("unexpected model: %v", got)
			}
		})
	}
}

func putObjectStorageListTestModels(t *testing.T) {
	t.Helper()
	put := func(id string, record map[string]interface{}) {
		if err := lkvstore.Put(id, record); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	put("os-list-src", map[string]interface{}{"id": "os-list-src", "isObjectStorageModel": true, "modelStage": "source"})
	put("os-list-plan", map[string]interface{}{"id": "os-list-plan", "isObjectStorageModel": true, "modelStage": "planning"})
	put("os-list-rdbms", map[string]interface{}{"id": "os-list-rdbms", "isRDBMSModel": true, "modelStage": "source"})
	put("os-list-sw", map[string]interface{}{"id": "os-list-sw", "isSoftwareModel": true, "modelStage": "source"})
	put("os-list-infra", map[string]interface{}{"id": "os-list-infra", "isCloudModel": false, "modelStage": "source"})
}

func TestGetStorageModelsFiltersCategoryAndStage(t *testing.T) {
	putObjectStorageListTestModels(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/object-storage-model?modelStage=source", nil)
	rec := httptest.NewRecorder()
	if err := GetObjectStorageModels(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"os-list-src"`) || strings.Contains(body, "os-list-plan") || strings.Contains(body, "os-list-rdbms") ||
		strings.Contains(body, "os-list-sw") || strings.Contains(body, "os-list-infra") {
		t.Fatalf("unexpected models: %s", body)
	}
}

func TestGetModelsObjectStorageCategory(t *testing.T) {
	putObjectStorageListTestModels(t)

	for _, query := range []string{"category=objectStorage", "category=objectstorage&modelStage=source"} {
		t.Run(query, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/model?"+query, nil)
			rec := httptest.NewRecorder()
			if err := GetModels(e.NewContext(req, rec)); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `"os-list-src"`) || strings.Contains(body, "os-list-rdbms") || strings.Contains(body, "os-list-sw") || strings.Contains(body, "os-list-infra") {
				t.Fatalf("unexpected models: %s", body)
			}
			if wantPlan := !strings.Contains(query, "modelStage"); strings.Contains(body, "os-list-plan") != wantPlan {
				t.Fatalf("planning model listed = %t, want %t: %s", !wantPlan, wantPlan, body)
			}
		})
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/model?category=blockStorage", nil)
	rec := httptest.NewRecorder()
	if err := GetModels(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "objectStorage") {
		t.Fatalf("status = %d, body = %s, want 400 listing 'objectStorage'", rec.Code, rec.Body)
	}
}

// A cm-beetle/imdl version older than v0.1.3 has no object storage model, so its API definition notes that instead
// (without loading the package).
func TestAddObjectStorageModelToImdlSwaggerDocBeforeMinVersion(t *testing.T) {
	paths := map[string]interface{}{
		"/object-storage-model":              map[string]interface{}{"post": map[string]interface{}{"description": "create"}},
		"/object-storage-model/{id}":         map[string]interface{}{"put": map[string]interface{}{"description": "update"}},
		"/object-storage-model/{id}/convert": map[string]interface{}{"get": map[string]interface{}{"description": "convert"}},
	}
	if err := addObjectStorageModelToImdlSwaggerDoc(t.Context(), map[string]interface{}{}, paths, "v0.1.2"); err != nil {
		t.Fatal(err)
	}
	for path, method := range map[string]string{"/object-storage-model": "post", "/object-storage-model/{id}": "put", "/object-storage-model/{id}/convert": "get"} {
		desc := paths[path].(map[string]interface{})[method].(map[string]interface{})["description"].(string)
		if strings.Count(desc, "has no object storage model. Use v0.1.3 or later.") != 1 {
			t.Fatalf("%s %s description = %q", method, path, desc)
		}
	}
}
