package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	echo "github.com/labstack/echo/v4"
)

func TestParseModelStagesParam(t *testing.T) {
	src, plan, tgt := StageSource, StagePlanning, StageTarget
	tests := []struct {
		query    string
		required bool
		want     []ModelStage
		wantErr  bool
	}{
		{query: "modelStage=source", want: []ModelStage{src}},
		{query: "modelStage=source,planning", want: []ModelStage{src, plan}},
		{query: "modelStage=source%2Cplanning", want: []ModelStage{src, plan}},
		{query: "modelStage=source&modelStage=planning", want: []ModelStage{src, plan}},
		{query: "modelStage=source,planning&modelStage=target", want: []ModelStage{src, plan, tgt}},
		{query: "modelStage=+Source+,+planning,source", want: []ModelStage{src, plan}},
		{query: "modelStage=target", required: true, want: []ModelStage{tgt}},
		{query: ""},
		{query: "modelStage="},
		{query: "modelStage=+"},
		{query: "", required: true, wantErr: true},
		{query: "modelStage=", required: true, wantErr: true},
		{query: "modelStage=source,foo", wantErr: true},
		{query: "modelStage=source&modelStage=foo", wantErr: true},
		{query: "modelStage=source,", wantErr: true},
		{query: "modelStage=,source", wantErr: true},
		{query: "modelStage=,", wantErr: true},
	}
	for _, tt := range tests {
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/model?"+tt.query, nil), httptest.NewRecorder())
		got, err := parseModelStagesParam(c, tt.required)
		if (err != nil) != tt.wantErr || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q (required=%t): got %v, %v, want %v (error: %t)", tt.query, tt.required, got, err, tt.want, tt.wantErr)
		}
	}
}

// listIDs returns the sorted IDs of the listed models which the test has put (with the prefix 'ms-').
func listIDs(t *testing.T, e *echo.Echo, target string) string {
	t.Helper()
	rec := request(t, e, http.MethodGet, target, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d: %s", target, rec.Code, rec.Body)
	}
	var models []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &models); err != nil || models == nil {
		t.Fatalf("GET %s: the response is not a list: %s", target, rec.Body)
	}
	ids := []string{}
	for _, m := range models {
		if id, _ := m["id"].(string); strings.HasPrefix(id, "ms-") {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func TestListModelsOfMultipleStages(t *testing.T) {
	e := setupUnifiedModelTest(t)
	lin := func(group, source string) map[string]interface{} {
		return map[string]interface{}{"sourceGroupId": group, "sourceModelId": source}
	}
	for prefix, flag := range map[string]map[string]interface{}{
		"ms-onprem": {"isCloudModel": false}, "ms-cloud": {"isCloudModel": true}, "ms-sw": {"isSoftwareModel": true},
		"ms-db": {"isRDBMSModel": true}, "ms-os": {"isObjectStorageModel": true},
	} {
		for _, stage := range modelStageOrder {
			m := map[string]interface{}{"modelStage": string(stage), "lineage": lin("g-"+prefix, prefix+"-source")}
			for k, v := range flag {
				m[k] = v
			}
			putTestModel(t, prefix+"-"+string(stage), m)
		}
	}

	for _, list := range []struct{ path, query, prefix string }{
		{"/infra-model", "modelType=onprem&", "ms-onprem"},
		{"/infra-model", "modelType=cloud&", "ms-cloud"},
		{"/software-model", "", "ms-sw"},
		{"/rdbms-model", "", "ms-db"},
		{"/object-storage-model", "", "ms-os"},
		{"/model", "category=software&", "ms-sw"},
		{"/model", "category=objectStorage&", "ms-os"},
	} {
		target := list.path + "?" + list.query
		src, plan, tgt := list.prefix+"-source", list.prefix+"-planning", list.prefix+"-target"
		for query, want := range map[string]string{
			"modelStage=source":                                      src,
			"modelStage=source,planning":                             plan + "," + src,
			"modelStage=source&modelStage=planning":                  plan + "," + src,
			"modelStage=source,planning,target":                      plan + "," + src + "," + tgt,
			"modelStage=planning,target&sourceModelId=" + src:        plan + "," + tgt,
			"modelStage=planning,target&sourceGroupId=no-such-group": "",
			"modelStage=planning&sourceGroupId=g-" + list.prefix:     plan,
		} {
			if got := listIDs(t, e, target+query); got != want {
				t.Errorf("GET %s%s: got [%s], want [%s]", target, query, got, want)
			}
		}
		for _, query := range []string{"modelStage=source,foo", "modelStage=source,", "modelStage=foo"} {
			if rec := request(t, e, http.MethodGet, target+query, ""); rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s%s: status = %d, want 400", target, query, rec.Code)
			}
		}
		// 'modelStage' is optional only for 'GET /model'.
		if list.path != "/model" {
			if rec := request(t, e, http.MethodGet, strings.TrimSuffix(target, "&"), ""); rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s: status = %d, want 400", target, rec.Code)
			}
		} else if got, want := listIDs(t, e, target), plan+","+src+","+tgt; got != want {
			t.Errorf("GET %s: got [%s], want [%s]", target, got, want)
		}
	}

	if got, want := listIDs(t, e, "/model?modelStage=source,planning&category=infra"), "ms-cloud-planning,ms-cloud-source,ms-onprem-planning,ms-onprem-source"; got != want {
		t.Errorf("got [%s], want [%s]", got, want)
	}

	// The APIs other than the list APIs take a single stage.
	for _, target := range []string{"/software-model/ms-sw-source", "/software-model/ms-sw-source/convert", "/rdbms-model/ms-db-source", "/object-storage-model/ms-os-source"} {
		if rec := request(t, e, http.MethodGet, target+"?modelStage=source,planning", ""); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestDecorateModel(t *testing.T) {
	tests := []struct {
		name  string
		model map[string]interface{}
		want  map[string]interface{} // nil: the field must not exist
	}{
		{"infra (on-premise)", map[string]interface{}{"isCloudModel": false, "modelStage": "source", "modelType": "OnPremiseModel"},
			map[string]interface{}{"category": "infra", "modelStage": "source", "modelType": "OnPremiseModel"}},
		{"infra (cloud, isTargetModel only)", map[string]interface{}{"isCloudModel": true, "isTargetModel": true, "modelType": "CloudModel"},
			map[string]interface{}{"category": "infra", "modelStage": "target", "modelType": "CloudModel", "isTargetModel": true}},
		{"software", map[string]interface{}{"isSoftwareModel": true, "modelStage": "planning", "modelType": "SoftwareModel"},
			map[string]interface{}{"category": "software", "modelStage": "planning", "modelType": "SoftwareModel"}},
		{"rdbms", map[string]interface{}{"isRDBMSModel": true, "modelStage": "target", "modelType": "RDBMSModel"},
			map[string]interface{}{"category": "rdbms", "modelType": "RDBMSModel"}},
		{"object storage (former modelType)", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "source", "modelType": "objectStorageModel"},
			map[string]interface{}{"category": "objectStorage", "modelType": "ObjectStorageModel"}},
		{"object storage", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "source", "modelType": "ObjectStorageModel"},
			map[string]interface{}{"category": "objectStorage", "modelType": "ObjectStorageModel"}},
		{"no category", map[string]interface{}{"modelStage": "source"},
			map[string]interface{}{"category": nil, "modelStage": "source"}},
		{"no stage", map[string]interface{}{"isCloudModel": false},
			map[string]interface{}{"category": "infra", "modelStage": nil, "lineage": nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, _ := json.Marshal(tt.model)
			got := decorateModel(tt.model)
			for field, want := range tt.want {
				if value, exists := got[field]; want == nil && exists || want != nil && value != want {
					t.Errorf("%s = %v, want %v", field, value, want)
				}
			}
			if _, exists := got["lineage"]; exists != (got["modelStage"] != nil) {
				t.Errorf("unexpected lineage: %v", got["lineage"])
			}
			if after, _ := json.Marshal(tt.model); string(after) != string(before) {
				t.Errorf("the stored map was modified: %s -> %s", before, after)
			}
		})
	}
}

// The large integers of the model body are responded as they are.
func TestDecorateRecordKeepsLargeIntegers(t *testing.T) {
	m, err := decorateRecord(onPremModelRecord{ModelStage: StageSource, OnpremInfraModel: json.RawMessage(`{"diskGB":18446744073709551615}`)})
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := json.Marshal(m); !strings.Contains(string(out), `"diskGB":18446744073709551615`) {
		t.Fatalf("the integer was changed: %s", out)
	}
}

// Every response of a model has 'category', which is never stored.
func TestResponsesHaveCategory(t *testing.T) {
	e := setupUnifiedModelTest(t)
	for _, tc := range unifiedModelCases() {
		t.Run(tc.name, func(t *testing.T) {
			check := func(what string, m map[string]interface{}) {
				t.Helper()
				if m["category"] != string(tc.category) || m["modelType"] != tc.wantType || m["modelStage"] != string(tc.stage) || m["lineage"] == nil {
					t.Errorf("%s: category = %v, modelType = %v, modelStage = %v, lineage = %v", what, m["category"], m["modelType"], m["modelStage"], m["lineage"])
				}
			}
			created := mustRequest(t, e, http.StatusCreated, http.MethodPost, tc.createTarget(), tc.reqBody("m1"))
			id := created["id"].(string)
			check("create", created)
			check("update", mustRequest(t, e, http.StatusOK, http.MethodPut, tc.idTarget(id, ""), tc.reqBody("m2")))
			check("get", mustRequest(t, e, http.StatusOK, http.MethodGet, tc.idTarget(id, ""), ""))
			check("convert", mustRequest(t, e, http.StatusOK, http.MethodGet, tc.idTarget(id, "/convert"), ""))

			for _, target := range []string{tc.createTarget(), "/model", "/model?category=" + string(tc.category)} {
				var models []map[string]interface{}
				if err := json.Unmarshal(request(t, e, http.MethodGet, target, "").Body.Bytes(), &models); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, m := range models {
					if m["category"] == nil || strings.Contains(target, "category=") && m["category"] != string(tc.category) {
						t.Errorf("GET %s: unexpected category: %v", target, m)
					}
					if m["id"] == id {
						found = true
						check("GET "+target, m)
					}
				}
				if !found {
					t.Errorf("GET %s: the model is not listed", target)
				}
			}

			stored := storedTestModel(t, id)
			if _, exists := stored["category"]; exists {
				t.Errorf("'category' must not be stored: %v", stored)
			}
			if stored["modelType"] != tc.wantType {
				t.Errorf("stored modelType = %v, want %s", stored["modelType"], tc.wantType)
			}
		})
	}
}

// The object storage models stored before v0.6.6 have 'objectStorageModel' as their 'modelType'.
func TestFormerObjectStorageModelType(t *testing.T) {
	e := setupUnifiedModelTest(t)
	putTestModel(t, "ms-os-former", map[string]interface{}{"isObjectStorageModel": true, "modelStage": "source", "modelType": "objectStorageModel",
		"objectStorageModelVersion": unifiedTestVersion, "sourceObjectStorageModel": map[string]interface{}{}})

	const path = "/object-storage-model/ms-os-former"
	for _, target := range []string{path + "?modelStage=source", path + "/convert?modelStage=source"} {
		if m := mustRequest(t, e, http.StatusOK, http.MethodGet, target, ""); m["modelType"] != "ObjectStorageModel" {
			t.Errorf("GET %s: modelType = %v", target, m["modelType"])
		}
	}
	for _, target := range []string{"/model", "/object-storage-model?modelStage=source"} {
		if body := request(t, e, http.MethodGet, target, "").Body.String(); strings.Contains(body, `"objectStorageModel"`) || !strings.Contains(body, `"ObjectStorageModel"`) {
			t.Errorf("GET %s: unexpected modelType: %s", target, body)
		}
	}
	if stored := storedTestModel(t, "ms-os-former"); stored["modelType"] != "objectStorageModel" {
		t.Errorf("the stored model was modified by reading: %v", stored)
	}

	mustRequest(t, e, http.StatusOK, http.MethodPut, path, `{"sourceObjectStorageModel":{}}`)
	if stored := storedTestModel(t, "ms-os-former"); stored["modelType"] != "ObjectStorageModel" {
		t.Errorf("stored modelType = %v", stored["modelType"])
	}
}

// 'GET /infra-model' returns an empty list rather than null, and only the models of the requested type.
func TestListInfraModelsWithoutMatch(t *testing.T) {
	e := setupUnifiedModelTest(t)
	putTestModel(t, "ms-no-stage", map[string]interface{}{"isCloudModel": false})
	rec := request(t, e, http.MethodGet, "/infra-model?modelType=onprem&modelStage=source,planning,target&sourceGroupId=no-such-group", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("status = %d, body = %s, want []", rec.Code, rec.Body)
	}
	// A model without a stage is not listed, but it can be read.
	if got := listIDs(t, e, "/infra-model?modelType=onprem&modelStage=source,planning,target"); got != "" {
		t.Errorf("got [%s], want no model", got)
	}
	if m := mustRequest(t, e, http.StatusOK, http.MethodGet, "/infra-model/ms-no-stage?modelType=onprem", ""); m["category"] != "infra" {
		t.Errorf("category = %v", m["category"])
	}
}
