package handler

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	echo "github.com/labstack/echo/v4"
)

// convertTestRDBMSSource has the 'rdbms-model' structs of cm-beetle/imdl v0.1.20 (the same since v0.1.18).
const convertTestRDBMSSource = `package rdbmsmodel
type CloudProperty struct {
	Csp    string ` + "`json:\"csp\"`" + `
	Region string ` + "`json:\"region\"`" + `
	Zone   string ` + "`json:\"zone,omitempty\"`" + `
}
type SourceRDBMS struct {
	Description          string                ` + "`json:\"description,omitempty\"`" + `
	SourceCloud          *CloudProperty        ` + "`json:\"sourceCloud,omitempty\"`" + `
	SourceRDBMSInstances []SourceRDBMSProperty ` + "`json:\"sourceRDBMSInstances\" validate:\"required,min=1\"`" + `
}
type SourceRDBMSProperty struct {
	DisplayName    string                  ` + "`json:\"displayName,omitempty\"`" + `
	Description    string                  ` + "`json:\"description,omitempty\"`" + `
	DBNode         DBNodeProperty          ` + "`json:\"dbNode\" validate:\"required\"`" + `
	DBEngine       DBEngineProperty        ` + "`json:\"dbEngine\" validate:\"required\"`" + `
	InnerDatabases []InnerDatabaseProperty ` + "`json:\"innerDatabases,omitempty\"`" + `
}
type DBNodeProperty struct {
	Hostname  string         ` + "`json:\"hostname,omitempty\"`" + `
	MachineId string         ` + "`json:\"machineId,omitempty\"`" + `
	CPU       CpuProperty    ` + "`json:\"cpu\" validate:\"required\"`" + `
	Memory    MemoryProperty ` + "`json:\"memory\" validate:\"required\"`" + `
	RootDisk  DiskProperty   ` + "`json:\"rootDisk,omitempty\"`" + `
	DataDisks []DiskProperty ` + "`json:\"dataDisks,omitempty\"`" + `
}
type CpuProperty struct {
	Architecture string  ` + "`json:\"architecture,omitempty\"`" + `
	Cpus         uint32  ` + "`json:\"cpus\" validate:\"required\"`" + `
	Cores        uint32  ` + "`json:\"cores\" validate:\"required\"`" + `
	Threads      uint32  ` + "`json:\"threads\" validate:\"required\"`" + `
	MaxSpeed     float32 ` + "`json:\"maxSpeed,omitempty\"`" + `
	Vendor       string  ` + "`json:\"vendor,omitempty\"`" + `
	Model        string  ` + "`json:\"model,omitempty\"`" + `
}
type MemoryProperty struct {
	Type      string ` + "`json:\"type,omitempty\"`" + `
	TotalSize uint64 ` + "`json:\"totalSize\" validate:\"required\"`" + `
	Available uint64 ` + "`json:\"available,omitempty\"`" + `
	Used      uint64 ` + "`json:\"used,omitempty\"`" + `
}
type DiskProperty struct {
	Label     string ` + "`json:\"label,omitempty\"`" + `
	Type      string ` + "`json:\"type\" validate:\"required\"`" + `
	TotalSize uint64 ` + "`json:\"totalSize\" validate:\"required\"`" + `
	Available uint64 ` + "`json:\"available,omitempty\"`" + `
	Used      uint64 ` + "`json:\"used,omitempty\"`" + `
}
type DBEngineProperty struct {
	Engine        string ` + "`json:\"engine\" validate:\"required\"`" + `
	EngineVersion string ` + "`json:\"engineVersion\" validate:\"required\"`" + `
	Port          int    ` + "`json:\"port,omitempty\"`" + `
	Role          string ` + "`json:\"role,omitempty\"`" + `
}
type InnerDatabaseProperty struct {
	DatabaseName string  ` + "`json:\"databaseName\" validate:\"required\"`" + `
	CharacterSet string  ` + "`json:\"characterSet,omitempty\"`" + `
	Collation    string  ` + "`json:\"collation,omitempty\"`" + `
	SizeMb       float64 ` + "`json:\"sizeMb,omitempty\"`" + `
	TableCount   int     ` + "`json:\"tableCount,omitempty\"`" + `
	RowCount     int64   ` + "`json:\"rowCount,omitempty\"`" + `
}
type RecommendedRDBMS struct {
	Status               string                ` + "`json:\"status\"`" + `
	Description          string                ` + "`json:\"description\"`" + `
	Warnings             []string              ` + "`json:\"warnings,omitempty\"`" + `
	TargetCloud          CloudProperty         ` + "`json:\"targetCloud\"`" + `
	TargetRDBMSInstances []TargetRDBMSInstance ` + "`json:\"targetRDBMSInstances\"`" + `
}
type TargetRDBMSInstance struct {
	SourceInstanceName        string           ` + "`json:\"sourceInstanceName\"`" + `
	RDBMSName                 string           ` + "`json:\"rdbmsName\" validate:\"required\"`" + `
	DBEngine                  string           ` + "`json:\"dbEngine\" validate:\"required\"`" + `
	StorageSize               int              ` + "`json:\"storageSize\"`" + `
	AdminUserName             string           ` + "`json:\"adminUserName\" validate:\"required\"`" + `
	AdminUserPassword         string           ` + "`json:\"adminUserPassword,omitempty\"`" + `
	NCPDBACGToAllowAllInbound bool             ` + "`json:\"ncpDBACGToAllowAllInbound,omitempty\"`" + `
	Databases                 []TargetDatabase ` + "`json:\"databases,omitempty\"`" + `
}
type TargetDatabase struct {
	DatabaseName string ` + "`json:\"databaseName\" validate:\"required\"`" + `
	Charset      string ` + "`json:\"charset,omitempty\"`" + `
	Collation    string ` + "`json:\"collation,omitempty\"`" + `
}
`

// convertTestSourceRDBMSV0113 is a source RDBMS model in the form of cm-beetle/imdl v0.1.13.
const convertTestSourceRDBMSV0113 = `{
	"description": "legacy",
	"sourceRDBMSInstances": [{
		"instanceName": "prod-mysql-01",
		"machineId": "m-1",
		"engine": "mysql",
		"engineVersion": "8.0",
		"vcpu": 4,
		"memoryMb": 8192,
		"storageSizeGb": 100,
		"storageType": "SSD",
		"iops": 3000,
		"port": 3306,
		"highAvailability": true,
		"publicAccess": false,
		"nhnDBSGToAllowAllInbound": false,
		"backupRetentionDays": 7,
		"databases": [{"databaseName": "order_db", "characterSet": "utf8mb4", "collation": "utf8mb4_unicode_ci", "sizeMb": 512.5, "tableCount": 24}]
	}]
}`

func pathOf(t *testing.T, v interface{}, keys ...interface{}) interface{} {
	t.Helper()
	for _, key := range keys {
		switch k := key.(type) {
		case string:
			m, ok := v.(map[string]interface{})
			if !ok {
				t.Fatalf("not an object at %v: %v", key, v)
			}
			v = m[k]
		case int:
			a, ok := v.([]interface{})
			if !ok || k >= len(a) {
				t.Fatalf("not an array (or too short) at %v: %v", key, v)
			}
			v = a[k]
		}
	}
	return v
}

func TestConvertSourceRDBMSModelFromV0113(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestRDBMSSource)

	for _, stage := range []ModelStage{StageSource, StagePlanning} {
		t.Run(string(stage), func(t *testing.T) {
			out, conversion, err := convertRDBMSModel(pkg, stage, "v0.1.13", "v0.1.20", []byte(convertTestSourceRDBMSV0113))
			if err != nil {
				t.Fatal(err)
			}
			if !conversion.Converted || conversion.OriginVersion != "v0.1.13" || conversion.CurrentVersion != "v0.1.20" {
				t.Fatalf("unexpected conversion info: %+v", conversion)
			}

			got := decodeJSON(t, out)
			instance := pathOf(t, got, "sourceRDBMSInstances", 0)
			expected := map[string]interface{}{
				"displayName": "prod-mysql-01",
				"dbEngine":    map[string]interface{}{"engine": "mysql", "engineVersion": "8.0", "port": float64(3306)},
				"dbNode": map[string]interface{}{
					"machineId": "m-1",
					"cpu":       map[string]interface{}{"cpus": float64(1), "cores": float64(4), "threads": float64(4)},
					"memory":    map[string]interface{}{"totalSize": float64(8)},
					"rootDisk":  map[string]interface{}{"type": "", "totalSize": float64(0)},
					"dataDisks": []interface{}{map[string]interface{}{"type": "SSD", "totalSize": float64(100)}},
				},
				"innerDatabases": []interface{}{map[string]interface{}{
					"databaseName": "order_db", "characterSet": "utf8mb4", "collation": "utf8mb4_unicode_ci", "sizeMb": 512.5, "tableCount": float64(24),
				}},
			}
			if !reflect.DeepEqual(instance, expected) {
				t.Fatalf("converted instance:\n got  %v\n want %v", instance, expected)
			}

			prefix := "sourceRDBMSModel.sourceRDBMSInstances[0]."
			wantDropped := []string{prefix + "backupRetentionDays", prefix + "highAvailability", prefix + "iops", prefix + "nhnDBSGToAllowAllInbound", prefix + "publicAccess"}
			dropped := append([]string(nil), conversion.DroppedFields...)
			sort.Strings(dropped)
			if !reflect.DeepEqual(dropped, wantDropped) {
				t.Fatalf("droppedFields = %v, want %v", dropped, wantDropped)
			}

			warnings := strings.Join(conversion.Warnings, "\n")
			for _, want := range []string{"vcpu' (4) was converted", "storageSizeGb' and 'storageType' were converted", "'" + prefix + "highAvailability' (true)", "'" + prefix + "iops' (3000)", "'" + prefix + "backupRetentionDays' (7)"} {
				if !strings.Contains(warnings, want) {
					t.Fatalf("warnings %v, want one containing %q", conversion.Warnings, want)
				}
			}
			// 8192 MB is exactly 8 GiB, and a false or zero value is not worth a warning.
			for _, unwanted := range []string{"memoryMb", "publicAccess", "nhnDBSGToAllowAllInbound"} {
				if strings.Contains(warnings, unwanted) {
					t.Fatalf("unexpected warning about %q: %v", unwanted, conversion.Warnings)
				}
			}
		})
	}
}

func TestConvertSourceRDBMSModelMemoryAndStorageEstimates(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestRDBMSSource)

	tests := []struct {
		name          string
		instance      string
		wantMemory    float64
		wantWarnings  []string
		wantNoDataDsk bool
	}{
		{"rounded memory", `{"instanceName":"a","engine":"mysql","engineVersion":"8.0","vcpu":2,"memoryMb":1536,"storageSizeGb":20,"storageType":"HDD"}`, 2, []string{"(1536 MB) was converted to 'dbNode.memory.totalSize' (2 GiB) by rounding"}, false},
		{"at least 1 GiB", `{"instanceName":"a","engine":"mysql","engineVersion":"8.0","vcpu":1,"memoryMb":300}`, 1, []string{"(300 MB) was converted to 'dbNode.memory.totalSize' (1 GiB)"}, true},
		{"storage without type", `{"instanceName":"a","engine":"mysql","engineVersion":"8.0","vcpu":1,"memoryMb":1024,"storageSizeGb":50}`, 1, []string{"dbNode.dataDisks[0].type' is required"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := `{"sourceRDBMSInstances":[` + tt.instance + `]}`
			out, conversion, err := convertRDBMSModel(pkg, StageSource, "v0.1.13", "v0.1.14", []byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			got := decodeJSON(t, out)
			if memory := pathOf(t, got, "sourceRDBMSInstances", 0, "dbNode", "memory", "totalSize"); memory != tt.wantMemory {
				t.Fatalf("memory.totalSize = %v, want %v", memory, tt.wantMemory)
			}
			if _, exists := pathOf(t, got, "sourceRDBMSInstances", 0, "dbNode").(map[string]interface{})["dataDisks"]; exists == tt.wantNoDataDsk {
				t.Fatalf("unexpected dataDisks: %v", got)
			}
			warnings := strings.Join(conversion.Warnings, "\n")
			for _, want := range tt.wantWarnings {
				if !strings.Contains(warnings, want) {
					t.Fatalf("warnings %v, want one containing %q", conversion.Warnings, want)
				}
			}
		})
	}
}

// The migrations of v0.1.14 apply only if the stored model is older than v0.1.14.
func TestRDBMSModelMigrationsApplyOnlyToVersionsInRange(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestRDBMSSource)
	raw := `{"sourceRDBMSInstances":[{"instanceName":"a","vcpu":2}]}`

	_, conversion, err := convertRDBMSModel(pkg, StageSource, "v0.1.14", "v0.1.20", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sourceRDBMSModel.sourceRDBMSInstances[0].instanceName", "sourceRDBMSModel.sourceRDBMSInstances[0].vcpu"}
	if !reflect.DeepEqual(conversion.DroppedFields, want) || len(conversion.Warnings) != 0 {
		t.Fatalf("droppedFields = %v, warnings = %v, want the old fields dropped without migration", conversion.DroppedFields, conversion.Warnings)
	}
}

// A model labeled with an older version can already have the content of a newer one; it must be kept as it is.
func TestConvertRDBMSModelKeepsContentNewerThanStoredVersion(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestRDBMSSource)
	raw := `{"sourceRDBMSInstances":[{"displayName":"a","dbNode":{"cpu":{"cpus":2,"cores":8,"threads":16},"memory":{"totalSize":64}},"dbEngine":{"engine":"mariadb","engineVersion":"10.5"}}]}`

	out, conversion, err := convertRDBMSModel(pkg, StageSource, "v0.1.13", "v0.1.20", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(conversion.DroppedFields) != 0 || len(conversion.Warnings) != 0 {
		t.Fatalf("unexpected conversion info: %+v", conversion)
	}
	got := decodeJSON(t, out)
	if cpu := pathOf(t, got, "sourceRDBMSInstances", 0, "dbNode", "cpu", "threads"); cpu != float64(16) {
		t.Fatalf("the stored cpu was not kept: %v", got)
	}
}

// An old field whose new place is already used is not moved, and is reported as a dropped field.
func TestConvertRDBMSModelReportsOldFieldWhenNewPlaceIsUsed(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestRDBMSSource)
	raw := `{"sourceRDBMSInstances":[{"engine":"mysql","dbEngine":{"engine":"mariadb","engineVersion":"10.5"},"vcpu":4,"dbNode":{"cpu":{"cpus":1,"cores":2,"threads":2}}}]}`

	out, conversion, err := convertRDBMSModel(pkg, StageSource, "v0.1.13", "v0.1.20", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sourceRDBMSModel.sourceRDBMSInstances[0].engine", "sourceRDBMSModel.sourceRDBMSInstances[0].vcpu"}
	if !reflect.DeepEqual(conversion.DroppedFields, want) {
		t.Fatalf("droppedFields = %v, want %v", conversion.DroppedFields, want)
	}
	got := decodeJSON(t, out)
	if engine := pathOf(t, got, "sourceRDBMSInstances", 0, "dbEngine", "engine"); engine != "mariadb" {
		t.Fatalf("the new field was overwritten: %v", got)
	}
}

func TestConvertTargetRDBMSModelCharacterSetToCharset(t *testing.T) {
	pkg := parseConvertTestPackage(t, convertTestRDBMSSource)
	raw := `{"status":"recommended","targetCloud":{"csp":"aws","region":"ap-northeast-2"},"targetRDBMSInstances":[{"rdbmsName":"r1","dbEngine":"mysql","adminUserName":"dbadmin","adminUserPassword":"Password123!","databases":[{"databaseName":"order_db","characterSet":"utf8mb4"}]}]}`

	tests := []struct {
		name        string
		origin      string
		wantCharset interface{}
		wantDropped []string
	}{
		{"older than v0.1.17", "v0.1.14", "utf8mb4", []string{}},
		// A v0.1.18 model cannot have 'characterSet', so it is dropped rather than renamed.
		{"v0.1.18 or later", "v0.1.18", nil, []string{"targetRDBMSModel.targetRDBMSInstances[0].databases[0].characterSet"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, conversion, err := convertRDBMSModel(pkg, StageTarget, tt.origin, "v0.1.20", []byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(conversion.DroppedFields, tt.wantDropped) {
				t.Fatalf("droppedFields = %v, want %v", conversion.DroppedFields, tt.wantDropped)
			}
			got := decodeJSON(t, out)
			database := pathOf(t, got, "targetRDBMSInstances", 0, "databases", 0).(map[string]interface{})
			if database["charset"] != tt.wantCharset {
				t.Fatalf("charset = %v, want %v: %v", database["charset"], tt.wantCharset, database)
			}
			if password := pathOf(t, got, "targetRDBMSInstances", 0, "adminUserPassword"); password != "Password123!" {
				t.Fatalf("adminUserPassword was not kept: %v", got)
			}
		})
	}
}

func TestValidateRDBMSModelConversion(t *testing.T) {
	versions := []string{"v0.1.13", "v0.1.14", "v0.1.19", "v0.1.20"}
	tests := []struct {
		name            string
		origin, current string
		wantErr         string
	}{
		{"to the latest version", "v0.1.13", "v0.1.20", ""},
		{"to a newer version", "v0.1.14", "v0.1.19", ""},
		{"to the same version", "v0.1.20", "v0.1.20", ""},
		{"to an older version", "v0.1.20", "v0.1.19", "only the conversion to a newer version"},
		{"stored version without the RDBMS model", "v0.1.12", "v0.1.20", "is not a cm-beetle/imdl tag with the RDBMS model"},
		{"stored version is empty", "", "v0.1.20", "is not a cm-beetle/imdl tag with the RDBMS model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRDBMSModelConversion(tt.origin, tt.current, versions)
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

func TestFilterRDBMSModelVersions(t *testing.T) {
	got := filterRDBMSModelVersions([]string{"v0.1.0", "v0.1.12", "v0.1.13", "v0.1.19", "v0.1.20"})
	if want := []string{"v0.1.13", "v0.1.19", "v0.1.20"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := filterRDBMSModelVersions([]string{"v0.1.12"}); got == nil || len(got) != 0 {
		t.Fatalf("got %v, want an empty list", got)
	}
}

func TestStoredModelCategoryRDBMS(t *testing.T) {
	if c, ok := storedModelCategory(map[string]interface{}{"isRDBMSModel": true}); !ok || c != categoryRDBMS {
		t.Fatalf("got (%s, %t), want (%s, true)", c, ok, categoryRDBMS)
	}
	if _, ok := storedModelCategory(map[string]interface{}{"isRDBMSModel": false}); ok {
		t.Fatal("a model with 'isRDBMSModel: false' and no other category must have no category")
	}
}

func setTestInfraModelVersions(t *testing.T, versions []string) {
	t.Helper()
	modelVersionCacheMu.Lock()
	previous, hadPrevious := modelVersionCache[infraModelTagSource.ModulePath]
	modelVersionCache[infraModelTagSource.ModulePath] = modelVersionCacheEntry{versions: versions, fetchedAt: time.Now()}
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
}

func requestConvertRDBMSModel(id, query string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/rdbms-model/"+id+"/convert?"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := ConvertRDBMSModel(c); err != nil {
		panic(err)
	}
	return rec
}

// The handler tests cover the responses which do not need the source code of a tagged version.
func TestConvertRDBMSModelHandler(t *testing.T) {
	setTestInfraModelVersions(t, []string{"v0.1.12", "v0.1.13", "v0.1.19", "v0.1.20"})

	put := func(id string, record map[string]interface{}) {
		if err := lkvstore.Put(id, record); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	body := map[string]interface{}{"sourceRDBMSInstances": []interface{}{}}
	put("rdbms-convert-latest", map[string]interface{}{"isRDBMSModel": true, "modelStage": "source", "rdbmsModelVersion": "v0.1.20", "sourceRDBMSModel": body})
	put("rdbms-convert-old", map[string]interface{}{"isRDBMSModel": true, "modelStage": "planning", "rdbmsModelVersion": "v0.1.19", "sourceRDBMSModel": body})
	put("rdbms-convert-pre-rdbms", map[string]interface{}{"isRDBMSModel": true, "modelStage": "source", "rdbmsModelVersion": "v0.1.12", "sourceRDBMSModel": body})
	put("rdbms-convert-noversion", map[string]interface{}{"isRDBMSModel": true, "modelStage": "source", "sourceRDBMSModel": body})
	put("rdbms-convert-nobody", map[string]interface{}{"isRDBMSModel": true, "modelStage": "target", "rdbmsModelVersion": "v0.1.13"})
	put("rdbms-convert-sw", map[string]interface{}{"isSoftwareModel": true, "modelStage": "source", "softwareModelVersion": "v0.1.4"})

	tests := []struct {
		name       string
		id, query  string
		wantStatus int
		wantText   string
	}{
		{"modelStage is required", "rdbms-convert-old", "", http.StatusBadRequest, "'modelStage' must be"},
		{"model not found", "rdbms-convert-no-such-model", "modelStage=source", http.StatusNotFound, "failed to find"},
		{"not an RDBMS model", "rdbms-convert-sw", "modelStage=source", http.StatusBadRequest, "is not an RDBMS model"},
		{"model stage mismatch", "rdbms-convert-old", "modelStage=source", http.StatusBadRequest, "model stage mismatch"},
		{"version is not a tag", "rdbms-convert-old", "modelStage=planning&rdbmsModelVersion=v9.9.9", http.StatusBadRequest, "not a tagged version"},
		{"version without the RDBMS model", "rdbms-convert-old", "modelStage=planning&rdbmsModelVersion=v0.1.12", http.StatusBadRequest, "not a tagged version"},
		{"downgrade", "rdbms-convert-old", "modelStage=planning&rdbmsModelVersion=v0.1.13", http.StatusBadRequest, "only the conversion to a newer version"},
		{"stored version without the RDBMS model", "rdbms-convert-pre-rdbms", "modelStage=source", http.StatusBadRequest, "is not a cm-beetle/imdl tag with the RDBMS model"},
		{"stored version is empty", "rdbms-convert-noversion", "modelStage=source", http.StatusBadRequest, "is not a cm-beetle/imdl tag with the RDBMS model"},
		{"stored model has no body", "rdbms-convert-nobody", "modelStage=target", http.StatusBadRequest, "no 'targetRDBMSModel'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertRDBMSModel(tt.id, tt.query)
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
		{"explicit same version", "modelStage=source&rdbmsModelVersion=v0.1.20"},
	}
	for _, tt := range sameVersionCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestConvertRDBMSModel("rdbms-convert-latest", tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			got := decodeJSON(t, rec.Body.Bytes())
			conversion := got["conversion"].(map[string]interface{})
			if conversion["converted"] != false || conversion["originVersion"] != "v0.1.20" || conversion["currentVersion"] != "v0.1.20" {
				t.Fatalf("unexpected conversion info: %v", conversion)
			}
			if got["rdbmsModelVersion"] != "v0.1.20" || got["modelStage"] != "source" {
				t.Fatalf("unexpected model: %v", got)
			}
		})
	}
}

func TestGetRDBMSModelsFiltersCategoryAndStage(t *testing.T) {
	put := func(id string, record map[string]interface{}) {
		if err := lkvstore.Put(id, record); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	put("rdbms-list-src", map[string]interface{}{"id": "rdbms-list-src", "isRDBMSModel": true, "modelStage": "source"})
	put("rdbms-list-plan", map[string]interface{}{"id": "rdbms-list-plan", "isRDBMSModel": true, "modelStage": "planning"})
	put("rdbms-list-sw", map[string]interface{}{"id": "rdbms-list-sw", "isSoftwareModel": true, "modelStage": "source"})
	put("rdbms-list-infra", map[string]interface{}{"id": "rdbms-list-infra", "isCloudModel": false, "modelStage": "source"})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/rdbms-model?modelStage=source", nil)
	rec := httptest.NewRecorder()
	if err := GetRDBMSModels(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"rdbms-list-src"`) || strings.Contains(body, "rdbms-list-plan") || strings.Contains(body, "rdbms-list-sw") || strings.Contains(body, "rdbms-list-infra") {
		t.Fatalf("unexpected models: %s", body)
	}
}
