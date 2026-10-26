package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cloud-barista/cm-beetle/imdl/lineage"
	model "github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	echo "github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// [Note]
// The migration lineage links the models derived from one another through the stages (source → planning → target).
// The lineage of a model refers only to its upstream models and itself, so that one model can be the parent of many
// models. The downstream models are found by filtering the list APIs with the lineage fields.
// (Refer to https://github.com/cloud-barista/cm-beetle/blob/main/docs/announcements/damselfly-model-metadata-lineage.md)

// ModelLineageReq holds the lineage fields of the request body for creating or updating a migration user model.
type ModelLineageReq struct {
	// ParentId is the ID of the model this model is derived from. It must be a model of the same category
	// (infra or software) in the previous stage: a source model for a planning model, and a planning model for a target model.
	ParentId string `json:"parentId,omitempty" example:"a4e3c3a6-7d2f-4a3e-9c1b-0f6f7a1d2e3b"`
	// Lineage is given to set the lineage explicitly (e.g. 'sourceGroupId'). The field of the model's own stage is set by the server.
	Lineage *lineage.MigrationLineage `json:"lineage,omitempty"`
}

// modelCategory is the category of a migration user model. The lineage links the models of the same category only.
type modelCategory string

const (
	categoryInfra    modelCategory = "infra"
	categorySoftware modelCategory = "software"
)

// storedModelCategory returns the category of a stored model.
func storedModelCategory(m map[string]interface{}) (modelCategory, bool) {
	if isSoftwareModel, ok := m["isSoftwareModel"].(bool); ok && isSoftwareModel {
		return categorySoftware, true
	}
	if _, ok := m["isCloudModel"].(bool); ok {
		return categoryInfra, true
	}
	return "", false
}

// modelStageOrder lists the model stages in the order in which models are derived.
var modelStageOrder = []ModelStage{StageSource, StagePlanning, StageTarget}

func (s ModelStage) order() int {
	for i, stage := range modelStageOrder {
		if stage == s {
			return i
		}
	}
	return -1
}

// previous returns the stage of the parent model, or "" for the source stage.
func (s ModelStage) previous() ModelStage {
	if i := s.order(); i > 0 {
		return modelStageOrder[i-1]
	}
	return ""
}

// lineageModelIdField returns the lineage field holding the ID of the model of the given stage, and its JSON name.
func lineageModelIdField(lin *lineage.MigrationLineage, stage ModelStage) (*string, string) {
	switch stage {
	case StageSource:
		return &lin.SourceModelId, "sourceModelId"
	case StagePlanning:
		return &lin.PlanningModelId, "planningModelId"
	case StageTarget:
		return &lin.TargetModelId, "targetModelId"
	}
	return nil, ""
}

// storedLineage returns the lineage of a stored model. The field of the model's own stage is filled in
// with its ID (for a model stored before the lineage was introduced).
func storedLineage(m map[string]interface{}, stage ModelStage) lineage.MigrationLineage {
	var lin lineage.MigrationLineage
	if raw, exists := m["lineage"]; exists && raw != nil {
		if b, err := json.Marshal(raw); err == nil {
			if err := json.Unmarshal(b, &lin); err != nil {
				log.Warn().Msgf("Invalid 'lineage' of the stored model [%v] : [%v]", m["id"], err)
			}
		}
	}
	if id, ok := m["id"].(string); ok && id != "" {
		if self, _ := lineageModelIdField(&lin, stage); self != nil && *self == "" {
			*self = id
		}
	}
	return lin
}

// withLineage returns the stored model with 'lineage' filled in (for a model stored before the lineage
// was introduced). The stored map is copied rather than modified.
func withLineage(m map[string]interface{}, stage ModelStage) map[string]interface{} {
	if _, exists := m["lineage"]; exists {
		return m
	}
	copied := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		copied[k] = v
	}
	copied["lineage"] = storedLineage(m, stage)
	return copied
}

// sourceGroupIdPattern keeps 'sourceGroupId' usable as a cloud resource label value ('cm-source-group-id'),
// following the strictest CSP rule (GCP: lowercase letters, digits, '_' and '-', up to 63 characters).
var sourceGroupIdPattern = regexp.MustCompile(`^[a-z0-9_-]{1,63}$`)

// modelLookup gets a stored model by ID. It is a parameter of resolveLineage for testing.
type modelLookup func(id string) (map[string]interface{}, bool)

func lookupStoredModel(id string) (map[string]interface{}, bool) {
	stored, exists := lkvstore.Get(id)
	if !exists {
		return nil, false
	}
	m, ok := stored.(map[string]interface{})
	return m, ok
}

// resolveLineage determines the lineage of the model 'selfId' of the given category and stage.
//   - 'base' is the stored lineage (empty on creation). It is preserved unless 'req.Lineage' is given, which replaces it.
//   - If 'req.ParentId' is given, the parent must be a model of the same category in the previous stage
//     (source → planning → target), and the upstream fields are derived from the parent.
//   - The field of the model's own stage is always set to 'selfId', and the fields of the downstream stages are left empty.
//
// On failure, it also returns the HTTP status code to respond with.
func resolveLineage(lookup modelLookup, selfId string, category modelCategory, stage ModelStage,
	base lineage.MigrationLineage, req ModelLineageReq) (lineage.MigrationLineage, int, error) {

	result := base
	explicit := req.Lineage != nil
	if explicit {
		result = *req.Lineage
	}
	result.SourceGroupId = strings.TrimSpace(result.SourceGroupId)
	for _, s := range modelStageOrder {
		field, _ := lineageModelIdField(&result, s)
		*field = strings.TrimSpace(*field)
	}

	// A model refers to itself only in the field of its own stage, and never to downstream models.
	// The fields carried over from the stored lineage (e.g. when the stage is changed) are dropped silently.
	for _, s := range modelStageOrder {
		field, name := lineageModelIdField(&result, s)
		if *field == "" || (s == stage && *field == selfId) {
			continue
		}
		switch {
		case s == stage || *field == selfId:
			if explicit {
				return result, http.StatusBadRequest, fmt.Errorf("invalid request: 'lineage.%s' (%s) cannot be set: the ID of the model itself is set by the server in the field of its own stage", name, *field)
			}
		case s.order() > stage.order():
			if explicit {
				return result, http.StatusBadRequest, fmt.Errorf("invalid request: 'lineage.%s' cannot be set for a %s model: the lineage refers only to upstream models", name, stage)
			}
		default:
			continue
		}
		log.Info().Msgf("Dropping 'lineage.%s' (%s) of the %s model [%s]", name, *field, stage, selfId)
		*field = ""
	}

	if explicit && result.SourceGroupId != "" && !sourceGroupIdPattern.MatchString(result.SourceGroupId) {
		return result, http.StatusBadRequest, fmt.Errorf("invalid request: 'lineage.sourceGroupId' (%s) must consist of 1 to 63 lowercase letters, digits, '_' or '-' to be used as a cloud resource label", result.SourceGroupId)
	}

	parentId := strings.TrimSpace(req.ParentId)
	if parentId != "" {
		if stage == StageSource {
			return result, http.StatusBadRequest, fmt.Errorf("invalid request: a source model cannot have a parent model ('parentId')")
		}
		if parentId == selfId {
			return result, http.StatusBadRequest, fmt.Errorf("invalid request: a model cannot be its own parent")
		}
		parent, _, status, err := getLineageModel(lookup, "parentId", parentId, category, stage.previous())
		if err != nil {
			return result, status, err
		}

		// The upstream fields are derived from the parent. A conflicting explicit value is rejected,
		// while a value carried over from the stored lineage is overwritten.
		parentLin := storedLineage(parent, stage.previous())
		type upstreamField struct {
			name           string
			value, derived *string
		}
		upstream := []upstreamField{{"sourceGroupId", &result.SourceGroupId, &parentLin.SourceGroupId}}
		for _, s := range modelStageOrder[:stage.order()] {
			field, name := lineageModelIdField(&result, s)
			derived, _ := lineageModelIdField(&parentLin, s)
			upstream = append(upstream, upstreamField{name, field, derived})
		}
		for _, u := range upstream {
			if *u.derived == "" || *u.value == *u.derived {
				continue
			}
			if explicit && *u.value != "" {
				return result, http.StatusBadRequest, fmt.Errorf("invalid request: 'lineage.%s' (%s) conflicts with that of the parent model [%s] (%s)", u.name, *u.value, parentId, *u.derived)
			}
			*u.value = *u.derived
		}
	}

	// A target model must be derived through a planning model.
	if stage == StageTarget && result.SourceModelId != "" && result.PlanningModelId == "" {
		return result, http.StatusBadRequest, fmt.Errorf("invalid request: a target model cannot skip the planning stage (source → planning → target). Specify 'parentId' of a planning model")
	}

	// The upstream models given by the request must exist and be consistent with one another, and the empty upstream
	// fields are derived from them. They are checked from the nearest stage so that the derived fields are checked as well.
	// (The stored lineage alone is not validated again, since it may refer to a model deleted with 'force'.)
	if explicit || parentId != "" {
		for i := stage.order() - 1; i >= 0; i-- {
			s := modelStageOrder[i]
			field, name := lineageModelIdField(&result, s)
			if *field == "" {
				continue
			}
			ref, refStage, status, err := getLineageModel(lookup, "lineage."+name, *field, category, s)
			if err != nil {
				return result, status, err
			}
			refLin := storedLineage(ref, refStage)
			if refLin.SourceGroupId != "" && result.SourceGroupId != "" && refLin.SourceGroupId != result.SourceGroupId {
				return result, http.StatusBadRequest, fmt.Errorf("invalid request: 'lineage.sourceGroupId' (%s) conflicts with that of the model [%s] (%s)", result.SourceGroupId, *field, refLin.SourceGroupId)
			}
			if result.SourceGroupId == "" {
				result.SourceGroupId = refLin.SourceGroupId
			}
			for _, upper := range modelStageOrder[:i] {
				value, upperName := lineageModelIdField(&result, upper)
				refValue, _ := lineageModelIdField(&refLin, upper)
				if *refValue != "" && *value != "" && *refValue != *value {
					return result, http.StatusBadRequest, fmt.Errorf("invalid request: 'lineage.%s' (%s) conflicts with that of the model [%s] (%s)", upperName, *value, *field, *refValue)
				}
				if *value == "" {
					*value = *refValue
				}
			}
		}
	}

	self, _ := lineageModelIdField(&result, stage)
	*self = selfId
	return result, 0, nil
}

// getLineageModel gets the model referred to by 'what' (e.g. 'parentId') and verifies its category and stage.
func getLineageModel(lookup modelLookup, what, id string, category modelCategory, stage ModelStage) (map[string]interface{}, ModelStage, int, error) {
	m, exists := lookup(id)
	if !exists {
		return nil, "", http.StatusNotFound, fmt.Errorf("the model given by '%s' [%s] does not exist", what, id)
	}
	if c, ok := storedModelCategory(m); !ok || c != category {
		return nil, "", http.StatusBadRequest, fmt.Errorf("invalid request: the model given by '%s' [%s] must be a model of the '%s' category", what, id, category)
	}
	s, ok := storedModelStage(m)
	if !ok || s != stage {
		return nil, "", http.StatusBadRequest, fmt.Errorf("invalid request: the model given by '%s' [%s] must be a %s model, but it is a '%s' model (stages: source → planning → target)", what, id, stage, s)
	}
	return m, s, 0, nil
}

// lineageReferrers returns the summaries of the models (except the model itself) whose lineage refers to the model 'id'.
func lineageReferrers(models []interface{}, id string) []interface{} {
	referrers := []interface{}{}
	for _, item := range models {
		m, ok := item.(map[string]interface{})
		if !ok || m["id"] == id {
			continue
		}
		stage, _ := storedModelStage(m)
		lin := storedLineage(m, stage)
		if lin.SourceModelId != id && lin.PlanningModelId != id && lin.TargetModelId != id {
			continue
		}
		category, _ := storedModelCategory(m)
		referrers = append(referrers, map[string]interface{}{
			"id":            m["id"],
			"category":      category,
			"modelStage":    stage,
			"userModelName": m["userModelName"],
		})
	}
	sort.Slice(referrers, func(i, j int) bool {
		return fmt.Sprint(referrers[i].(map[string]interface{})["id"]) < fmt.Sprint(referrers[j].(map[string]interface{})["id"])
	})
	return referrers
}

// findLineageReferrers returns the summaries of the stored models whose lineage refers to the model 'id'.
func findLineageReferrers(id string) []interface{} {
	models, _ := lkvstore.GetWithPrefix("")
	return lineageReferrers(models, id)
}

// parseForceParam parses the optional 'force' query parameter.
func parseForceParam(c echo.Context) (bool, error) {
	param := strings.TrimSpace(c.QueryParam("force"))
	if param == "" {
		return false, nil
	}
	force, err := strconv.ParseBool(param)
	if err != nil {
		return false, fmt.Errorf("invalid request: 'force' must be 'true' or 'false', got '%s'", param)
	}
	return force, nil
}

// lineageConflictResp is the response for deleting (or changing the stage of) a model referred to by other models.
func lineageConflictResp(c echo.Context, id, action string, referrers []interface{}) error {
	newErr := fmt.Errorf("cannot %s the model [%s]: it is referred to by the lineage of %d model(s) listed in 'list'", action, id, len(referrers))
	log.Warn().Msg(newErr.Error())
	return c.JSON(http.StatusConflict, model.Response{Success: false, Text: newErr.Error(), List: referrers})
}

// parseLineageFilter parses the optional lineage filter query parameters of the list APIs.
func parseLineageFilter(c echo.Context) lineage.MigrationLineage {
	return lineage.MigrationLineage{
		SourceGroupId:   strings.TrimSpace(c.QueryParam("sourceGroupId")),
		SourceModelId:   strings.TrimSpace(c.QueryParam("sourceModelId")),
		PlanningModelId: strings.TrimSpace(c.QueryParam("planningModelId")),
		TargetModelId:   strings.TrimSpace(c.QueryParam("targetModelId")),
	}
}

// matchLineageFilter reports whether the lineage has every non-empty field of the filter.
func matchLineageFilter(lin, filter lineage.MigrationLineage) bool {
	return (filter.SourceGroupId == "" || filter.SourceGroupId == lin.SourceGroupId) &&
		(filter.SourceModelId == "" || filter.SourceModelId == lin.SourceModelId) &&
		(filter.PlanningModelId == "" || filter.PlanningModelId == lin.PlanningModelId) &&
		(filter.TargetModelId == "" || filter.TargetModelId == lin.TargetModelId)
}
