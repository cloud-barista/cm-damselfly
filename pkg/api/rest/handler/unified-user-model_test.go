package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	echo "github.com/labstack/echo/v4"
)

// unifiedTestVersion and unifiedTestOldVersion are the model versions served by setupUnifiedModelTest. They are not
// versions of the other tests, because the parsed packages are cached by version.
const (
	unifiedTestVersion    = "v9.0.1"
	unifiedTestOldVersion = "v9.0.0"
)

// setupUnifiedModelTest returns a server with the unified APIs which needs no network access: the tagged versions are
// given by the cache, and the source code of unifiedTestVersion is served by a local Go module proxy.
func setupUnifiedModelTest(t *testing.T) *echo.Echo {
	t.Helper()

	setTestModelVersions(t, infraModelTagSource.ModulePath, unifiedTestOldVersion, unifiedTestVersion)
	setTestModelVersions(t, softwareModelTagSource.ModulePath, unifiedTestOldVersion, unifiedTestVersion)

	modules := map[string]map[string]string{
		infraModelTagSource.ModulePath: {
			"on-premise-model/model.go": convertTestOnpremSource,
			"cloud-model/model.go":      convertTestCloudSource,
			"rdbms-model/model.go":      convertTestRDBMSSource,
			"storage-model/model.go":    convertTestObjectStorageSource,
		},
		softwareModelTagSource.ModulePath: {"model.go": convertTestSmdlSource},
	}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for modulePath, files := range modules {
			version := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"+modulePath+"/@v/"), ".zip")
			if version != unifiedTestVersion && version != unifiedTestOldVersion {
				continue
			}
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for name, content := range files {
				f, err := zw.Create(modulePath + "@" + version + "/" + name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				_, _ = f.Write([]byte(content))
			}
			_ = zw.Close()
			_, _ = w.Write(buf.Bytes())
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(proxy.Close)
	t.Setenv("GOPROXY", proxy.URL)

	lkvstore.Init(lkvstore.Config{DbFilePath: filepath.Join(t.TempDir(), "test.db")})

	e := echo.New()
	e.GET("/model", GetModels)
	e.GET("/infra-model", GetInfraModels)
	e.POST("/infra-model", CreateInfraModel)
	e.GET("/infra-model/:id", GetInfraModel)
	e.GET("/infra-model/:id/convert", ConvertInfraModel)
	e.PUT("/infra-model/:id", UpdateInfraModel)
	e.DELETE("/infra-model/:id", DeleteInfraModel)
	e.GET("/software-model", GetSoftwareModels)
	e.POST("/software-model", CreateSoftwareModel)
	e.GET("/software-model/:id", GetSoftwareModel)
	e.GET("/software-model/:id/convert", ConvertSoftwareModel)
	e.PUT("/software-model/:id", UpdateSoftwareModel)
	e.DELETE("/software-model/:id", DeleteSoftwareModel)
	e.GET("/rdbms-model", GetRDBMSModels)
	e.POST("/rdbms-model", CreateRDBMSModel)
	e.GET("/rdbms-model/:id", GetRDBMSModel)
	e.GET("/rdbms-model/:id/convert", ConvertRDBMSModel)
	e.PUT("/rdbms-model/:id", UpdateRDBMSModel)
	e.DELETE("/rdbms-model/:id", DeleteRDBMSModel)
	e.GET("/object-storage-model", GetObjectStorageModels)
	e.POST("/object-storage-model", CreateObjectStorageModel)
	e.GET("/object-storage-model/:id", GetObjectStorageModel)
	e.GET("/object-storage-model/:id/convert", ConvertObjectStorageModel)
	e.PUT("/object-storage-model/:id", UpdateObjectStorageModel)
	e.DELETE("/object-storage-model/:id", DeleteObjectStorageModel)
	return e
}

// setTestModelVersions sets the tagged versions of a model module during the test.
func setTestModelVersions(t *testing.T, modulePath string, versions ...string) {
	t.Helper()
	modelVersionCacheMu.Lock()
	defer modelVersionCacheMu.Unlock()
	previous, hadPrevious := modelVersionCache[modulePath]
	modelVersionCache[modulePath] = modelVersionCacheEntry{versions: versions, fetchedAt: time.Now()}
	t.Cleanup(func() {
		modelVersionCacheMu.Lock()
		defer modelVersionCacheMu.Unlock()
		if hadPrevious {
			modelVersionCache[modulePath] = previous
		} else {
			delete(modelVersionCache, modulePath)
		}
	})
}

// putTestModel puts a model to the lkvstore during the test.
func putTestModel(t *testing.T, id string, record map[string]interface{}) {
	t.Helper()
	record["id"] = id
	if err := lkvstore.Put(id, record); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lkvstore.Delete(id) })
}

// request sends a request to the server. The models created by the request are deleted after the test.
func request(t *testing.T, e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusCreated {
		id, _ := decodeJSON(t, rec.Body.Bytes())["id"].(string)
		t.Cleanup(func() { lkvstore.Delete(id) })
	}
	return rec
}

// mustRequest sends a request which must succeed with the given status, and returns the model in the response.
func mustRequest(t *testing.T, e *echo.Echo, wantStatus int, method, target, body string) map[string]interface{} {
	t.Helper()
	rec := request(t, e, method, target, body)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d: %s", method, target, rec.Code, wantStatus, rec.Body)
	}
	return decodeJSON(t, rec.Body.Bytes())
}

// storedTestModel returns the model as it is stored in the lkvstore.
func storedTestModel(t *testing.T, id string) map[string]interface{} {
	t.Helper()
	stored, exists := lkvstore.Get(id)
	if !exists {
		t.Fatalf("the model [%s] is not stored", id)
	}
	return stored.(map[string]interface{})
}

// unifiedModelCase is a kind of model which the unified APIs create.
type unifiedModelCase struct {
	name     string
	path     string
	query    string // the query parameters other than 'modelStage'
	category modelCategory
	stage    ModelStage
	field    string // the field of the model body
	body     string
	wantType string
}

func unifiedModelCases() []unifiedModelCase {
	onprem, cloud := `{"nodes":[]}`, `{"targetInfra":{"name":"infra01"}}`
	swSource, swTarget := `{"source_group_id":"g1","connection_info_list":[]}`, `{"servers":[]}`
	return []unifiedModelCase{
		{"infra source", "/infra-model", "modelType=onprem", categoryInfra, StageSource, "onpremiseInfraModel", onprem, "OnPremiseModel"},
		{"infra planning", "/infra-model", "modelType=onprem", categoryInfra, StagePlanning, "onpremiseInfraModel", onprem, "OnPremiseModel"},
		{"infra target", "/infra-model", "modelType=cloud", categoryInfra, StageTarget, "cloudInfraModel", cloud, "CloudModel"},
		{"software source", "/software-model", "", categorySoftware, StageSource, "sourceSoftwareModel", swSource, "SoftwareModel"},
		{"software planning", "/software-model", "", categorySoftware, StagePlanning, "sourceSoftwareModel", swSource, "SoftwareModel"},
		{"software target", "/software-model", "", categorySoftware, StageTarget, "targetSoftwareModel", swTarget, "SoftwareModel"},
		{"rdbms source", "/rdbms-model", "", categoryRDBMS, StageSource, "sourceRDBMSModel", `{}`, "RDBMSModel"},
		{"rdbms target", "/rdbms-model", "", categoryRDBMS, StageTarget, "targetRDBMSModel", `{}`, "RDBMSModel"},
		{"object storage source", "/object-storage-model", "", categoryObjectStorage, StageSource, "sourceObjectStorageModel", `{}`, "ObjectStorageModel"},
		{"object storage target", "/object-storage-model", "", categoryObjectStorage, StageTarget, "targetObjectStorageModel", `{}`, "ObjectStorageModel"},
	}
}

func (c unifiedModelCase) createTarget() string {
	return strings.TrimSuffix(fmt.Sprintf("%s?modelStage=%s&%s", c.path, c.stage, c.query), "&")
}

// idTarget returns the target of a single model API (e.g. suffix '/convert').
func (c unifiedModelCase) idTarget(id, suffix string) string {
	return strings.TrimSuffix(fmt.Sprintf("%s/%s%s?modelStage=%s&%s", c.path, id, suffix, c.stage, c.query), "&")
}

func (c unifiedModelCase) reqBody(name string) string {
	return fmt.Sprintf(`{"userId":"u1","userModelName":%q,"userModelVersion":"v1","description":"d1","isInitUserModel":true,%q:%s}`, name, c.field, c.body)
}

func sortedKeys(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// 'isTargetModel' was only for the legacy APIs, so it is not stored any more.
func TestCreateAndUpdateDoNotStoreIsTargetModel(t *testing.T) {
	e := setupUnifiedModelTest(t)
	for _, tc := range unifiedModelCases() {
		t.Run(tc.name, func(t *testing.T) {
			created := mustRequest(t, e, http.StatusCreated, http.MethodPost, tc.createTarget(), tc.reqBody("m1"))
			id := created["id"].(string)
			updated := mustRequest(t, e, http.StatusOK, http.MethodPut, tc.idTarget(id, ""), tc.reqBody("m2"))
			for what, m := range map[string]map[string]interface{}{"create response": created, "update response": updated, "stored": storedTestModel(t, id)} {
				if _, exists := m["isTargetModel"]; exists {
					t.Errorf("%s has 'isTargetModel': %v", what, m)
				}
				if m["modelStage"] != string(tc.stage) {
					t.Errorf("%s: modelStage = %v, want %s", what, m["modelStage"], tc.stage)
				}
			}
			if updated["userModelName"] != "m2" {
				t.Errorf("the model was not updated: %v", updated)
			}
		})
	}
}

// The JSON keys of the stored models must not be changed, since they are read by the other versions of cm-damselfly.
func TestStoredModelKeys(t *testing.T) {
	e := setupUnifiedModelTest(t)
	common := "createTime,description,id,isInitUserModel,lineage,modelStage,modelType,updateTime,userId,userModelName,userModelVersion"
	want := map[string]string{
		"infra source":          "isCloudModel,onpremModelVersion,onpremiseInfraModel",
		"infra planning":        "isCloudModel,onpremModelVersion,onpremiseInfraModel",
		"infra target":          "cloudInfraModel,cloudModelVersion,csp,isCloudModel,region,zone",
		"software source":       "isSoftwareModel,softwareModelVersion,sourceSoftwareModel",
		"software planning":     "isSoftwareModel,softwareModelVersion,sourceSoftwareModel",
		"software target":       "isSoftwareModel,softwareModelVersion,targetSoftwareModel",
		"rdbms source":          "isRDBMSModel,rdbmsModelVersion,sourceRDBMSModel",
		"rdbms target":          "isRDBMSModel,rdbmsModelVersion,targetRDBMSModel",
		"object storage source": "isObjectStorageModel,objectStorageModelVersion,sourceObjectStorageModel",
		"object storage target": "isObjectStorageModel,objectStorageModelVersion,targetObjectStorageModel",
	}
	for _, tc := range unifiedModelCases() {
		t.Run(tc.name, func(t *testing.T) {
			keys := strings.Split(common+","+want[tc.name], ",")
			sort.Strings(keys)
			created := mustRequest(t, e, http.StatusCreated, http.MethodPost, tc.createTarget(), tc.reqBody("m1"))
			if got := sortedKeys(storedTestModel(t, created["id"].(string))); got != strings.Join(keys, ",") {
				t.Fatalf("stored keys\n got: %s\nwant: %s", got, strings.Join(keys, ","))
			}
		})
	}
}

// A model stored by an older version has 'isTargetModel' without 'modelStage'. Its stage is still derived from it, and
// 'isTargetModel' is removed when the model is updated.
func TestLegacyRecordWithIsTargetModelOnly(t *testing.T) {
	e := setupUnifiedModelTest(t)
	putTestModel(t, "legacy-onprem", map[string]interface{}{"isCloudModel": false, "isTargetModel": false, "modelType": "OnPremiseModel", "onpremModelVersion": "v0.0.3", "createTime": "2025-01-01 00:00:00"})
	putTestModel(t, "legacy-cloud", map[string]interface{}{"isCloudModel": true, "isTargetModel": true, "modelType": "CloudModel", "cloudModelVersion": "v0.9.16"})
	putTestModel(t, "legacy-sw-target", map[string]interface{}{"isSoftwareModel": true, "isTargetModel": true, "modelType": "SoftwareModel", "targetSoftwareModel": map[string]interface{}{"servers": nil}})

	for target, wantStage := range map[string]string{
		"/infra-model/legacy-onprem?modelType=onprem":        "source",
		"/infra-model/legacy-cloud?modelType=cloud":          "target",
		"/software-model/legacy-sw-target?modelStage=target": "target",
	} {
		if m := mustRequest(t, e, http.StatusOK, http.MethodGet, target, ""); m["modelStage"] != wantStage {
			t.Errorf("GET %s: modelStage = %v, want %s", target, m["modelStage"], wantStage)
		}
	}

	// The stored version is not a tag, so the body is validated against the built-in struct.
	mustRequest(t, e, http.StatusOK, http.MethodPut, "/infra-model/legacy-onprem?modelType=onprem", `{"userModelName":"renamed","onpremiseInfraModel":{}}`)
	mustRequest(t, e, http.StatusOK, http.MethodPut, "/software-model/legacy-sw-target", `{"userModelName":"renamed","targetSoftwareModel":{"servers":[]}}`)
	for id, wantStage := range map[string]string{"legacy-onprem": "source", "legacy-sw-target": "target"} {
		stored := storedTestModel(t, id)
		if _, exists := stored["isTargetModel"]; exists || stored["modelStage"] != wantStage || stored["userModelName"] != "renamed" {
			t.Errorf("unexpected stored model: %v", stored)
		}
	}
	if stored := storedTestModel(t, "legacy-onprem"); stored["createTime"] != "2025-01-01 00:00:00" {
		t.Errorf("'createTime' was not preserved: %v", stored)
	}
}
