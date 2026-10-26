package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/cloud-barista/cm-beetle/imdl/lineage"
)

// storedModel builds a model as it is read from the lkvstore (a JSON-decoded map).
func storedModel(t *testing.T, id string, category modelCategory, stage ModelStage, lin *lineage.MigrationLineage) map[string]interface{} {
	t.Helper()
	m := map[string]interface{}{"id": id, "modelStage": string(stage), "userModelName": id}
	if category == categorySoftware {
		m["isSoftwareModel"] = true
	} else {
		m["isCloudModel"] = stage == StageTarget
	}
	if lin != nil {
		b, _ := json.Marshal(lin)
		var raw map[string]interface{}
		_ = json.Unmarshal(b, &raw)
		m["lineage"] = raw
	}
	return m
}

func testLookup(models ...map[string]interface{}) modelLookup {
	byId := map[string]map[string]interface{}{}
	for _, m := range models {
		byId[m["id"].(string)] = m
	}
	return func(id string) (map[string]interface{}, bool) {
		m, ok := byId[id]
		return m, ok
	}
}

func TestResolveLineage(t *testing.T) {
	src := storedModel(t, "src-1", categoryInfra, StageSource, &lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1"})
	src2 := storedModel(t, "src-2", categoryInfra, StageSource, &lineage.MigrationLineage{SourceGroupId: "sg-2", SourceModelId: "src-2"})
	legacySrc := storedModel(t, "src-legacy", categoryInfra, StageSource, nil)
	plan := storedModel(t, "plan-1", categoryInfra, StagePlanning, &lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"})
	tgt := storedModel(t, "tgt-1", categoryInfra, StageTarget, &lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1", TargetModelId: "tgt-1"})
	swSrc := storedModel(t, "sw-src-1", categorySoftware, StageSource, nil)
	lookup := testLookup(src, src2, legacySrc, plan, tgt, swSrc)

	tests := []struct {
		name       string
		selfId     string
		category   modelCategory
		stage      ModelStage
		base       lineage.MigrationLineage
		req        ModelLineageReq
		want       lineage.MigrationLineage
		wantStatus int
		wantErr    string
	}{
		{
			name: "source without lineage", selfId: "new", category: categoryInfra, stage: StageSource,
			want: lineage.MigrationLineage{SourceModelId: "new"},
		},
		{
			name: "source with explicit sourceGroupId", selfId: "new", category: categoryInfra, stage: StageSource,
			req:  ModelLineageReq{Lineage: &lineage.MigrationLineage{SourceGroupId: " sg-x "}},
			want: lineage.MigrationLineage{SourceGroupId: "sg-x", SourceModelId: "new"},
		},
		{
			name: "source cannot have a parent", selfId: "new", category: categoryInfra, stage: StageSource,
			req:        ModelLineageReq{ParentId: "src-1"},
			wantStatus: http.StatusBadRequest, wantErr: "cannot have a parent",
		},
		{
			name: "planning derived from source", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:  ModelLineageReq{ParentId: "src-1"},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "new"},
		},
		{
			name: "planning derived from a legacy source without lineage", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:  ModelLineageReq{ParentId: "src-legacy"},
			want: lineage.MigrationLineage{SourceModelId: "src-legacy", PlanningModelId: "new"},
		},
		{
			name: "target derived from planning", selfId: "new", category: categoryInfra, stage: StageTarget,
			req:  ModelLineageReq{ParentId: "plan-1"},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1", TargetModelId: "new"},
		},
		{
			name: "target cannot be derived from source (planning skipped)", selfId: "new", category: categoryInfra, stage: StageTarget,
			req:        ModelLineageReq{ParentId: "src-1"},
			wantStatus: http.StatusBadRequest, wantErr: "must be a planning model",
		},
		{
			name: "target cannot skip planning with explicit lineage", selfId: "new", category: categoryInfra, stage: StageTarget,
			req:        ModelLineageReq{Lineage: &lineage.MigrationLineage{SourceModelId: "src-1"}},
			wantStatus: http.StatusBadRequest, wantErr: "cannot skip the planning stage",
		},
		{
			name: "planning cannot be derived from target", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{ParentId: "tgt-1"},
			wantStatus: http.StatusBadRequest, wantErr: "must be a source model",
		},
		{
			name: "planning cannot be derived from planning", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{ParentId: "plan-1"},
			wantStatus: http.StatusBadRequest, wantErr: "must be a source model",
		},
		{
			name: "parent not found", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{ParentId: "nope"},
			wantStatus: http.StatusNotFound, wantErr: "does not exist",
		},
		{
			name: "parent of another category", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{ParentId: "sw-src-1"},
			wantStatus: http.StatusBadRequest, wantErr: "must be a model of the 'infra' category",
		},
		{
			name: "explicit lineage conflicting with the parent", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{ParentId: "src-1", Lineage: &lineage.MigrationLineage{SourceGroupId: "sg-other"}},
			wantStatus: http.StatusBadRequest, wantErr: "conflicts with that of the parent",
		},
		{
			name: "explicit lineage referring to a model of a wrong stage", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{Lineage: &lineage.MigrationLineage{SourceModelId: "plan-1"}},
			wantStatus: http.StatusBadRequest, wantErr: "must be a source model",
		},
		{
			name: "explicit lineage inconsistent with the referred planning model", selfId: "new", category: categoryInfra, stage: StageTarget,
			req:        ModelLineageReq{Lineage: &lineage.MigrationLineage{SourceModelId: "src-2", PlanningModelId: "plan-1"}},
			wantStatus: http.StatusBadRequest, wantErr: "conflicts with that of the model [plan-1]",
		},
		{
			name: "explicit planningModelId derives the upstream fields from that model", selfId: "new", category: categoryInfra, stage: StageTarget,
			req:  ModelLineageReq{Lineage: &lineage.MigrationLineage{PlanningModelId: "plan-1"}},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1", TargetModelId: "new"},
		},
		{
			name: "explicit downstream field", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{Lineage: &lineage.MigrationLineage{TargetModelId: "tgt-1"}},
			wantStatus: http.StatusBadRequest, wantErr: "refers only to upstream models",
		},
		{
			name: "explicit self field with another ID", selfId: "new", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{Lineage: &lineage.MigrationLineage{PlanningModelId: "plan-1"}},
			wantStatus: http.StatusBadRequest, wantErr: "set by the server",
		},
		{
			name: "explicit self field with its own ID (as returned by GET)", selfId: "plan-1", category: categoryInfra, stage: StagePlanning,
			req:  ModelLineageReq{Lineage: &lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"}},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"},
		},
		{
			name: "invalid sourceGroupId as a label value", selfId: "new", category: categoryInfra, stage: StageSource,
			req:        ModelLineageReq{Lineage: &lineage.MigrationLineage{SourceGroupId: "SG 1"}},
			wantStatus: http.StatusBadRequest, wantErr: "lowercase letters",
		},
		{
			name: "update: the stored lineage is preserved", selfId: "plan-1", category: categoryInfra, stage: StagePlanning,
			base: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"},
		},
		{
			name: "update: the stored lineage referring to a deleted model is not validated again", selfId: "plan-1", category: categoryInfra, stage: StagePlanning,
			base: lineage.MigrationLineage{SourceModelId: "deleted", PlanningModelId: "plan-1"},
			want: lineage.MigrationLineage{SourceModelId: "deleted", PlanningModelId: "plan-1"},
		},
		{
			name: "update: explicit lineage replaces the stored one", selfId: "src-x", category: categoryInfra, stage: StageSource,
			base: lineage.MigrationLineage{SourceGroupId: "sg-old", SourceModelId: "src-x"},
			req:  ModelLineageReq{Lineage: &lineage.MigrationLineage{}},
			want: lineage.MigrationLineage{SourceModelId: "src-x"},
		},
		{
			name: "update: a new parent overwrites the stored upstream fields", selfId: "plan-x", category: categoryInfra, stage: StagePlanning,
			base: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-x"},
			req:  ModelLineageReq{ParentId: "src-2"},
			want: lineage.MigrationLineage{SourceGroupId: "sg-2", SourceModelId: "src-2", PlanningModelId: "plan-x"},
		},
		{
			name: "update: stage source -> planning moves the own field", selfId: "m-1", category: categoryInfra, stage: StagePlanning,
			base: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "m-1"},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", PlanningModelId: "m-1"},
		},
		{
			name: "update: stage planning -> source drops the upstream field", selfId: "m-1", category: categoryInfra, stage: StageSource,
			base: lineage.MigrationLineage{SourceModelId: "src-1", PlanningModelId: "m-1"},
			want: lineage.MigrationLineage{SourceModelId: "m-1"},
		},
		{
			name: "update: stage planning -> target without a planning parent skips planning", selfId: "m-1", category: categoryInfra, stage: StageTarget,
			base:       lineage.MigrationLineage{SourceModelId: "src-1", PlanningModelId: "m-1"},
			wantStatus: http.StatusBadRequest, wantErr: "cannot skip the planning stage",
		},
		{
			name: "update: stage planning -> target with a planning parent", selfId: "m-1", category: categoryInfra, stage: StageTarget,
			base: lineage.MigrationLineage{SourceModelId: "src-1", PlanningModelId: "m-1"},
			req:  ModelLineageReq{ParentId: "plan-1"},
			want: lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1", TargetModelId: "m-1"},
		},
		{
			name: "a model cannot be its own parent", selfId: "plan-1", category: categoryInfra, stage: StagePlanning,
			req:        ModelLineageReq{ParentId: "plan-1"},
			wantStatus: http.StatusBadRequest, wantErr: "its own parent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, status, err := resolveLineage(lookup, tt.selfId, tt.category, tt.stage, tt.base, tt.req)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got lineage %+v", tt.wantErr, got)
				}
				if status != tt.wantStatus || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got status %d, error %q; want status %d, error containing %q", status, err, tt.wantStatus, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStoredLineageAndWithLineage(t *testing.T) {
	legacy := storedModel(t, "m-1", categoryInfra, StagePlanning, nil)
	if got := storedLineage(legacy, StagePlanning); got != (lineage.MigrationLineage{PlanningModelId: "m-1"}) {
		t.Fatalf("unexpected lineage of a legacy model: %+v", got)
	}
	filled := withLineage(legacy, StagePlanning)
	if _, exists := legacy["lineage"]; exists {
		t.Fatal("withLineage must not modify the stored map")
	}
	if filled["lineage"] != (lineage.MigrationLineage{PlanningModelId: "m-1"}) {
		t.Fatalf("unexpected filled lineage: %+v", filled["lineage"])
	}

	stored := storedModel(t, "m-2", categoryInfra, StageTarget, &lineage.MigrationLineage{SourceModelId: "s", PlanningModelId: "p", TargetModelId: "m-2"})
	if got := storedLineage(stored, StageTarget); got != (lineage.MigrationLineage{SourceModelId: "s", PlanningModelId: "p", TargetModelId: "m-2"}) {
		t.Fatalf("unexpected stored lineage: %+v", got)
	}
}

func TestLineageReferrers(t *testing.T) {
	models := []interface{}{
		storedModel(t, "src-1", categoryInfra, StageSource, &lineage.MigrationLineage{SourceModelId: "src-1"}),
		storedModel(t, "plan-2", categoryInfra, StagePlanning, &lineage.MigrationLineage{SourceModelId: "src-1", PlanningModelId: "plan-2"}),
		storedModel(t, "plan-1", categoryInfra, StagePlanning, &lineage.MigrationLineage{SourceModelId: "src-1", PlanningModelId: "plan-1"}),
		storedModel(t, "tgt-1", categoryInfra, StageTarget, &lineage.MigrationLineage{SourceModelId: "src-1", PlanningModelId: "plan-1", TargetModelId: "tgt-1"}),
		storedModel(t, "other", categorySoftware, StageSource, nil),
		"not a map",
	}

	ids := func(referrers []interface{}) []string {
		var result []string
		for _, r := range referrers {
			result = append(result, r.(map[string]interface{})["id"].(string))
		}
		return result
	}

	if got := strings.Join(ids(lineageReferrers(models, "src-1")), ","); got != "plan-1,plan-2,tgt-1" {
		t.Fatalf("referrers of src-1: got %s", got)
	}
	if got := strings.Join(ids(lineageReferrers(models, "plan-1")), ","); got != "tgt-1" {
		t.Fatalf("referrers of plan-1: got %s", got)
	}
	if got := lineageReferrers(models, "tgt-1"); len(got) != 0 {
		t.Fatalf("a target model is referred to by no model, got %v", got)
	}
	if got := lineageReferrers(models, "other"); len(got) != 0 {
		t.Fatalf("a legacy model is referred to by no model, got %v", got)
	}
}

func TestMatchLineageFilter(t *testing.T) {
	lin := lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"}
	if !matchLineageFilter(lin, lineage.MigrationLineage{}) {
		t.Fatal("an empty filter must match")
	}
	if !matchLineageFilter(lin, lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1"}) {
		t.Fatal("a matching filter must match")
	}
	if matchLineageFilter(lin, lineage.MigrationLineage{TargetModelId: "tgt-1"}) {
		t.Fatal("a non-matching filter must not match")
	}
}

func TestFilterModels(t *testing.T) {
	legacyTarget := storedModel(t, "legacy-tgt", categoryInfra, StageTarget, nil)
	delete(legacyTarget, "modelStage")
	legacyTarget["isTargetModel"] = true

	models := []interface{}{
		storedModel(t, "src-1", categoryInfra, StageSource, &lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1"}),
		storedModel(t, "plan-1", categoryInfra, StagePlanning, &lineage.MigrationLineage{SourceGroupId: "sg-1", SourceModelId: "src-1", PlanningModelId: "plan-1"}),
		storedModel(t, "sw-plan-1", categorySoftware, StagePlanning, &lineage.MigrationLineage{SourceGroupId: "sg-1", PlanningModelId: "sw-plan-1"}),
		storedModel(t, "sw-src-2", categorySoftware, StageSource, &lineage.MigrationLineage{SourceGroupId: "sg-2", SourceModelId: "sw-src-2"}),
		legacyTarget,
		map[string]interface{}{"id": "no-category", "modelStage": "source"},
		map[string]interface{}{"id": "no-stage", "isCloudModel": false},
		"not a map",
	}

	ids := func(result []map[string]interface{}) string {
		var list []string
		for _, m := range result {
			list = append(list, m["id"].(string))
		}
		sort.Strings(list)
		return strings.Join(list, ",")
	}

	tests := []struct {
		name     string
		stage    ModelStage
		category modelCategory
		filter   lineage.MigrationLineage
		want     string
	}{
		{name: "all stages and categories", want: "legacy-tgt,plan-1,src-1,sw-plan-1,sw-src-2"},
		{name: "planning of both categories", stage: StagePlanning, want: "plan-1,sw-plan-1"},
		{name: "software of all stages", category: categorySoftware, want: "sw-plan-1,sw-src-2"},
		{name: "target (legacy record with isTargetModel only)", stage: StageTarget, want: "legacy-tgt"},
		{name: "source group across stages and categories", filter: lineage.MigrationLineage{SourceGroupId: "sg-1"}, want: "plan-1,src-1,sw-plan-1"},
		{name: "derived from a source model (including itself)", filter: lineage.MigrationLineage{SourceModelId: "src-1"}, want: "plan-1,src-1"},
		{name: "infra planning in a source group", stage: StagePlanning, category: categoryInfra, filter: lineage.MigrationLineage{SourceGroupId: "sg-1"}, want: "plan-1"},
		{name: "no match", filter: lineage.MigrationLineage{SourceGroupId: "sg-none"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterModels(models, tt.stage, tt.category, tt.filter)
			if result == nil {
				t.Fatal("the result must be an empty list rather than nil")
			}
			if got := ids(result); got != tt.want {
				t.Fatalf("got [%s], want [%s]", got, tt.want)
			}
		})
	}

	// 'modelStage' and 'lineage' are filled in for the legacy record, without modifying the stored map.
	result := filterModels([]interface{}{legacyTarget}, "", "", lineage.MigrationLineage{})
	if result[0]["modelStage"] != "target" || result[0]["lineage"] != (lineage.MigrationLineage{TargetModelId: "legacy-tgt"}) {
		t.Fatalf("unexpected filled fields: modelStage=%v, lineage=%v", result[0]["modelStage"], result[0]["lineage"])
	}
	if _, exists := legacyTarget["lineage"]; exists {
		t.Fatal("filterModels must not modify the stored map")
	}
}
