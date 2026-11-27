package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestSoftwareModelFieldOfStage(t *testing.T) {
	req := &softwareModelRawReq{SourceSoftwareModel: []byte(`"source"`), TargetSoftwareModel: []byte(`"target"`)}
	for stage, want := range map[ModelStage]struct {
		schema softwareModelSchemaInfo
		raw    string
	}{
		StageSource:   {sourceSoftwareModelSchema, `"source"`},
		StagePlanning: {sourceSoftwareModelSchema, `"source"`},
		StageTarget:   {targetSoftwareModelSchema, `"target"`},
	} {
		if got := softwareModelSchemaOf(stage); got != want.schema {
			t.Errorf("softwareModelSchemaOf(%s) = %v, want %v", stage, got, want.schema)
		}
		if got := string(req.rawSoftwareModel(stage)); got != want.raw {
			t.Errorf("rawSoftwareModel(%s) = %s, want %s", stage, got, want.raw)
		}
	}
}

// A software planning model has the source structure, so that the lineage is linked through source → planning → target.
func TestSoftwareModelChain(t *testing.T) {
	e := setupUnifiedModelTest(t)
	sourceBody := `{"source_group_id":"g1","connection_info_list":[]}`

	source := mustRequest(t, e, http.StatusCreated, http.MethodPost, "/software-model?modelStage=source",
		`{"userModelName":"src","lineage":{"sourceGroupId":"sg-01"},"sourceSoftwareModel":`+sourceBody+`}`)
	planning := mustRequest(t, e, http.StatusCreated, http.MethodPost, "/software-model?modelStage=planning",
		fmt.Sprintf(`{"userModelName":"plan","parentId":%q,"sourceSoftwareModel":%s}`, source["id"], sourceBody))
	target := mustRequest(t, e, http.StatusCreated, http.MethodPost, "/software-model?modelStage=target",
		fmt.Sprintf(`{"userModelName":"tgt","parentId":%q,"targetSoftwareModel":{"servers":[]}}`, planning["id"]))

	if _, exists := planning["targetSoftwareModel"]; exists || planning["sourceSoftwareModel"] == nil {
		t.Fatalf("the planning model must have 'sourceSoftwareModel' only: %v", planning)
	}
	lin := target["lineage"].(map[string]interface{})
	for field, want := range map[string]interface{}{
		"sourceGroupId": "sg-01", "sourceModelId": source["id"], "planningModelId": planning["id"], "targetModelId": target["id"],
	} {
		if lin[field] != want {
			t.Errorf("lineage.%s = %v, want %v", field, lin[field], want)
		}
	}
}

func TestSoftwareModelRequiresFieldOfStage(t *testing.T) {
	e := setupUnifiedModelTest(t)
	source, target := `{"sourceSoftwareModel":{"source_group_id":"g1","connection_info_list":[]}}`, `{"targetSoftwareModel":{"servers":[]}}`
	for _, tc := range []struct {
		stage, body, wantText string
	}{
		{"source", target, "'sourceSoftwareModel' required"},
		{"planning", target, "'sourceSoftwareModel' required"},
		{"target", source, "'targetSoftwareModel' required"},
	} {
		rec := request(t, e, http.MethodPost, "/software-model?modelStage="+tc.stage, tc.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.wantText) {
			t.Errorf("%s: status = %d, body = %s, want 400 with %q", tc.stage, rec.Code, rec.Body, tc.wantText)
		}
	}
}

// The planning models stored with 'targetSoftwareModel' before v0.6.6.
func TestFormerPlanningSoftwareModel(t *testing.T) {
	e := setupUnifiedModelTest(t)
	put := func(id, version string) {
		putTestModel(t, id, map[string]interface{}{
			"isSoftwareModel": true, "modelStage": "planning", "modelType": "SoftwareModel", "softwareModelVersion": version,
			"targetSoftwareModel": map[string]interface{}{"servers": []interface{}{}},
		})
	}
	put("former-plan", unifiedTestVersion)
	put("former-plan-old", unifiedTestOldVersion)
	put("former-plan-untagged", "untagged")
	path := "/software-model/former-plan"

	// Read as it is
	if m := mustRequest(t, e, http.StatusOK, http.MethodGet, path+"?modelStage=planning", ""); m["targetSoftwareModel"] == nil {
		t.Errorf("unexpected model: %v", m)
	}
	if rec := request(t, e, http.MethodGet, "/software-model?modelStage=planning", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"former-plan"`) {
		t.Errorf("the model is not listed: %d %s", rec.Code, rec.Body)
	}

	// Not converted to any version
	for _, target := range []string{
		path + "/convert?modelStage=planning",
		path + "/convert?modelStage=planning&modelVersion=" + unifiedTestVersion,
		"/software-model/former-plan-old/convert?modelStage=planning&modelVersion=" + unifiedTestVersion,
		"/software-model/former-plan-old/convert?modelStage=planning&modelVersion=" + unifiedTestOldVersion,
		"/software-model/former-plan-untagged/convert?modelStage=planning",
	} {
		rec := request(t, e, http.MethodGet, target, "")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "former target structure") {
			t.Errorf("GET %s: status = %d, body = %s, want 400", target, rec.Code, rec.Body)
		}
	}

	// Updated with the source structure only
	rec := request(t, e, http.MethodPut, path, `{"targetSoftwareModel":{"servers":[]}}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "'sourceSoftwareModel' required") {
		t.Errorf("status = %d, body = %s, want 400", rec.Code, rec.Body)
	}
	sourceBody := `{"userModelName":"plan","sourceSoftwareModel":{"source_group_id":"g1","connection_info_list":[]}}`
	for _, id := range []string{"former-plan", "former-plan-untagged"} {
		mustRequest(t, e, http.StatusOK, http.MethodPut, "/software-model/"+id, sourceBody)
		stored := storedTestModel(t, id)
		body, _ := stored["sourceSoftwareModel"].(map[string]interface{})
		if _, exists := stored["targetSoftwareModel"]; exists || body["source_group_id"] != "g1" || stored["modelStage"] != "planning" {
			t.Errorf("unexpected stored model: %v", stored)
		}
	}
	mustRequest(t, e, http.StatusOK, http.MethodGet, path+"/convert?modelStage=planning", "")

	// Changed to a target model, or deleted
	mustRequest(t, e, http.StatusOK, http.MethodPut, "/software-model/former-plan-old?modelStage=target", `{"targetSoftwareModel":{"servers":[]}}`)
	mustRequest(t, e, http.StatusOK, http.MethodGet, "/software-model/former-plan-old?modelStage=target", "")
	put("former-plan", unifiedTestVersion)
	if rec := request(t, e, http.MethodDelete, path+"?modelStage=planning", ""); rec.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
}
