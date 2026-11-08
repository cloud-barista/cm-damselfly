package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloud-barista/cm-beetle/imdl/lineage"
	rdbmsmodel "github.com/cloud-barista/cm-beetle/imdl/rdbms-model"
	model "github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	"github.com/google/uuid"
	echo "github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ##############################################################################################
// ### RDBMS Migration User Model (Source + Planning + Target)
// ##############################################################################################

// [Note]
// An RDBMS migration user model follows the struct of the 'rdbms-model' package of a cm-beetle/imdl tagged version
// (v0.1.13 or later), which is given by 'rdbmsModelVersion':
//   - A source or planning model has 'sourceRDBMSModel' ('SourceRDBMS' struct, the input of the cm-beetle RDBMS recommendation).
//   - A target model has 'targetRDBMSModel' ('RecommendedRDBMS' struct, the input of the cm-beetle RDBMS migration).
// The lineage links the RDBMS models only (category 'rdbms').

// CreateRDBMSModelReq is the request body for creating or updating an RDBMS migration user model.
// Only the field matching the 'modelStage' query parameter needs to be provided
// ('sourceRDBMSModel' for source or planning, 'targetRDBMSModel' for target).
type CreateRDBMSModelReq struct {
	UserId          string `json:"userId"`
	NodeId          string `json:"nodeId,omitempty"`
	IsInitUserModel bool   `json:"isInitUserModel"`
	UserModelName   string `json:"userModelName"`
	UserModelVer    string `json:"userModelVersion"`
	Description     string `json:"description"`
	ModelLineageReq
	SourceRDBMSModel *rdbmsmodel.SourceRDBMS      `json:"sourceRDBMSModel,omitempty"`
	TargetRDBMSModel *rdbmsmodel.RecommendedRDBMS `json:"targetRDBMSModel,omitempty"`
}

// [Note]
// rdbmsModelRawReq is the actual request body of CreateRDBMSModel/UpdateRDBMSModel. The RDBMS model is kept as
// raw JSON (shadowing the typed fields of CreateRDBMSModelReq) so that it can be validated against the struct of
// the selected cm-beetle/imdl tagged version instead of the version compiled into cm-damselfly.
type rdbmsModelRawReq struct {
	CreateRDBMSModelReq
	SourceRDBMSModel json.RawMessage `json:"sourceRDBMSModel,omitempty"`
	TargetRDBMSModel json.RawMessage `json:"targetRDBMSModel,omitempty"`
}

// rawRDBMSModel returns the raw RDBMS model field matching the model stage.
func (r *rdbmsModelRawReq) rawRDBMSModel(stage ModelStage) json.RawMessage {
	if stage == StageTarget {
		return r.TargetRDBMSModel
	}
	return r.SourceRDBMSModel
}

// RDBMSModelRespInfo holds the metadata of a stored RDBMS migration user model.
type RDBMSModelRespInfo struct {
	Id              string                   `json:"id"`
	UserId          string                   `json:"userId"`
	NodeId          string                   `json:"nodeId,omitempty"`
	IsInitUserModel bool                     `json:"isInitUserModel"`
	UserModelName   string                   `json:"userModelName"`
	UserModelVer    string                   `json:"userModelVersion"`
	Description     string                   `json:"description"`
	RDBMSModelVer   string                   `json:"rdbmsModelVersion"`
	CreateTime      string                   `json:"createTime"`
	UpdateTime      string                   `json:"updateTime"`
	IsRDBMSModel    bool                     `json:"isRDBMSModel"`
	ModelType       string                   `json:"modelType"`
	ModelStage      ModelStage               `json:"modelStage"`
	Lineage         lineage.MigrationLineage `json:"lineage"`
}

// sourceRDBMSModelRecord and targetRDBMSModelRecord are the stored forms of RDBMS models whose
// 'sourceRDBMSModel' / 'targetRDBMSModel' follow the struct of their own cm-beetle/imdl version.
// A source or planning model is stored as sourceRDBMSModelRecord, and a target model as targetRDBMSModelRecord.
type sourceRDBMSModelRecord struct {
	RDBMSModelRespInfo
	SourceRDBMSModel json.RawMessage `json:"sourceRDBMSModel"`
}

type targetRDBMSModelRecord struct {
	RDBMSModelRespInfo
	TargetRDBMSModel json.RawMessage `json:"targetRDBMSModel"`
}

func rdbmsModelErrResp(c echo.Context, status int, err error) error {
	if status >= http.StatusInternalServerError {
		log.Error().Msg(err.Error())
	} else {
		log.Warn().Msg(err.Error())
	}
	return c.JSON(status, model.Response{Success: false, Text: err.Error()})
}

// isStoredRDBMSModel reports whether the stored model is an RDBMS model.
func isStoredRDBMSModel(m map[string]interface{}) bool {
	isRDBMSModel, ok := m["isRDBMSModel"].(bool)
	return ok && isRDBMSModel
}

// getRDBMSModelFromStore gets the RDBMS model with the given ID and verifies that it is an RDBMS
// model of the given stage (if 'stage' is empty, only that it is an RDBMS model with a valid stage).
// It also returns the stored stage, and on failure, the HTTP status code to respond with.
func getRDBMSModelFromStore(id string, stage ModelStage) (map[string]interface{}, ModelStage, int, error) {
	stored, exists := lkvstore.Get(id)
	if !exists {
		return nil, "", http.StatusNotFound, fmt.Errorf("failed to find the model from db with id: [%s]", id)
	}

	m, ok := stored.(map[string]interface{})
	if !ok {
		return nil, "", http.StatusInternalServerError, fmt.Errorf("internal error: unexpected model data format")
	}

	if !isStoredRDBMSModel(m) {
		return nil, "", http.StatusBadRequest, fmt.Errorf("invalid request: the model with id [%s] is not an RDBMS model", id)
	}

	storedStage, ok := storedModelStage(m)
	if !ok {
		return nil, "", http.StatusBadRequest, fmt.Errorf("the model with id [%s] has no valid 'modelStage'", id)
	}
	if stage != "" && storedStage != stage {
		return nil, "", http.StatusBadRequest, fmt.Errorf("model stage mismatch: the RDBMS model with id [%s] is a '%s' model, not '%s'",
			id, storedStage, stage)
	}

	return withLineage(withModelStage(m, storedStage), storedStage), storedStage, http.StatusOK, nil
}

// saveRDBMSModelToStore puts the model to the key-value store and saves the store to file.
func saveRDBMSModelToStore(id string, userModel interface{}) error {
	if err := lkvstore.Put(id, userModel); err != nil {
		return fmt.Errorf("failed to put the model to the lkvstore : [%v]", err)
	}
	if err := lkvstore.SaveLkvStore(); err != nil {
		return fmt.Errorf("failed to save the lkvstore to file : [%v]", err)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	return nil
}

// GetRDBMSModels godoc
// @ID GetRDBMSModels
// @Summary Get a list of RDBMS migration user models (source, planning or target)
// @Description Get a list of RDBMS migration user models. Use 'modelStage' to select the stage (source, planning or target) of the models.
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceModelId' to find the models derived from a source model).
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelStage query string true "Stage of RDBMS model to retrieve" Enums(source, planning, target)
// @Param sourceGroupId   query string false "Filter by 'lineage.sourceGroupId'"
// @Param sourceModelId   query string false "Filter by 'lineage.sourceModelId'"
// @Param planningModelId query string false "Filter by 'lineage.planningModelId'"
// @Param targetModelId   query string false "Filter by 'lineage.targetModelId'"
// @Success 200 {array}  map[string]interface{} "Successfully obtained RDBMS migration user models"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 500 {object} model.Response
// @Router /rdbms-model [get]
func GetRDBMSModels(c echo.Context) error {
	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}
	lineageFilter := parseLineageFilter(c)
	log.Info().Msgf("# GetRDBMSModels: modelStage=[%s], lineage=[%+v]", modelStage, lineageFilter)

	modelList, _ := lkvstore.GetWithPrefix("")
	return c.JSON(http.StatusOK, filterModels(modelList, modelStage, categoryRDBMS, lineageFilter))
}

// GetRDBMSModel godoc
// @ID GetRDBMSModel
// @Summary Get a specific RDBMS migration user model (source, planning or target)
// @Description Get a specific RDBMS migration user model by ID. Use 'modelStage' to specify the stage (source, planning or target) of the model.
// @Description The 'lineage' of the model refers to its upstream models and itself.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string true "Stage of the RDBMS model" Enums(source, planning, target)
// @Success 200 {object} object "Successfully obtained the RDBMS migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /rdbms-model/{id} [get]
func GetRDBMSModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# GetRDBMSModel: id=[%s], modelStage=[%s]", id, modelStage)

	m, _, status, err := getRDBMSModelFromStore(id, modelStage)
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	return c.JSON(http.StatusOK, m)
}

// ConvertRDBMSModel godoc
// @ID ConvertRDBMSModel
// @Summary Convert a stored RDBMS migration user model to a newer model version
// @Description Get a stored RDBMS migration user model converted to the struct of the cm-beetle/imdl tagged version given by 'rdbmsModelVersion'. The stored model is not changed.
// @Description Only the conversion to the same or a newer version is allowed, and a model whose stored version is not a cm-beetle/imdl tag with the RDBMS model (v0.1.13 or later) cannot be converted.
// @Description The response has the fields of 'GET /rdbms-model/{id}', with 'rdbmsModelVersion' and 'sourceRDBMSModel' (or 'targetRDBMSModel') converted, and 'conversion' added: 'originVersion' (the stored version), 'currentVersion' (the version of the returned model), 'converted' (false if the stored version is already the requested version), 'droppedFields' (the paths of the fields removed because the requested version does not have them) and 'warnings' (what could not be converted as it was).
// @Description [Migrations] v0.1.14: the instances of 'sourceRDBMSModel' were restructured into 'dbNode' and 'dbEngine' ('vcpu', 'memoryMb' and 'storageSizeGb' are converted as estimates and reported in 'warnings'). v0.1.18: 'characterSet' of the target databases is converted to 'charset'.
// @Description If a field of the stored model has a value which does not match the type of that field in the requested version, the model is not converted (400).
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id                path  string true  "Model ID"
// @Param modelStage        query string true  "Stage of the RDBMS model" Enums(source, planning, target)
// @Param rdbmsModelVersion query string false "Model version to convert to (a cm-beetle 'imdl/vX.Y.Z' tag of v0.1.13 or later, e.g. v0.1.20). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Success 200 {object} object "Successfully obtained the converted RDBMS migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter, or the model cannot be converted to the version"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /rdbms-model/{id}/convert [get]
func ConvertRDBMSModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# ConvertRDBMSModel: id=[%s], modelStage=[%s], rdbmsModelVersion=[%s]", id, modelStage, c.QueryParam("rdbmsModelVersion"))

	m, _, status, err := getRDBMSModelFromStore(id, modelStage)
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	rdbmsVersions, err := getRDBMSModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		return rdbmsModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	currentVersion, err := selectModelVersion(c.QueryParam("rdbmsModelVersion"), rdbmsVersions)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid RDBMS model version: %w", err))
	}

	originVersion, _ := m["rdbmsModelVersion"].(string)
	if err := validateRDBMSModelConversion(originVersion, currentVersion, rdbmsVersions); err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}

	converted := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		converted[k] = v
	}

	if originVersion == currentVersion {
		converted["conversion"] = newRDBMSModelConversion(originVersion, currentVersion, false)
		return c.JSON(http.StatusOK, converted)
	}

	schema := rdbmsModelSchemaOf(modelStage)
	if m[schema.FieldName] == nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("the stored model with id [%s] has no '%s' to convert", id, schema.FieldName))
	}
	rawRDBMSModel, err := json.Marshal(m[schema.FieldName])
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to read the stored '%s' : [%v]", schema.FieldName, err))
	}

	pkg, err := modelschema.LoadPackage(c.Request().Context(), infraModelSchemaSource, currentVersion, rdbmsModelPkgDir)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to load the cm-beetle/imdl %s '%s' model : [%v]", currentVersion, rdbmsModelPkgDir, err))
	}
	if !pkg.HasType(schema.RootType) {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("'%s' struct does not exist in cm-beetle/imdl %s '%s'. Use another version", schema.RootType, currentVersion, rdbmsModelPkgDir))
	}

	rdbmsModel, conversion, err := convertRDBMSModel(pkg, modelStage, originVersion, currentVersion, rawRDBMSModel)
	if err != nil {
		newErr := fmt.Errorf("failed to convert the model with id [%s] from %s to %s : [%v]", id, originVersion, currentVersion, err)
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return rdbmsModelErrResp(c, http.StatusBadRequest, newErr)
		}
		return rdbmsModelErrResp(c, http.StatusInternalServerError, newErr)
	}
	log.Info().Msgf("Converted the model [%s] from %s to %s (dropped fields: %d, warnings: %d)", id, originVersion, currentVersion, len(conversion.DroppedFields), len(conversion.Warnings))

	converted["rdbmsModelVersion"] = currentVersion
	converted[schema.FieldName] = rdbmsModel
	converted["conversion"] = conversion
	return c.JSON(http.StatusOK, converted)
}

// CreateRDBMSModel godoc
// @ID CreateRDBMSModel
// @Summary Create a new RDBMS migration user model (source, planning or target)
// @Description Create a new RDBMS migration user model. Use 'modelStage' to select the stage (source, planning or target) of the model.
// @Description Provide 'sourceRDBMSModel' for a source or planning model (modelStage=source or planning), or 'targetRDBMSModel' for a target model (modelStage=target).
// @Description The 'sourceRDBMSModel' (or 'targetRDBMSModel') is validated against the 'SourceRDBMS' (or 'RecommendedRDBMS') struct of the cm-beetle/imdl tagged version given by 'rdbmsModelVersion', and is stored in that struct's form.
// @Description 'nodeId' is an optional identifier of the node the model is associated with.
// @Description [Lineage] 'parentId' is the ID of the RDBMS model this model is derived from, which must be in the previous stage (source → planning → target): a source model for a planning model, and a planning model for a target model. The upstream fields of 'lineage' are derived from the parent.
// @Description 'lineage' can also be given explicitly (e.g. 'sourceGroupId'). The field of the model's own stage (e.g. 'planningModelId' of a planning model) is set to the ID of the created model by the server.
// @Description (The body schema below shows the struct of the cm-beetle/imdl version built into cm-damselfly. To see the struct of another version, choose 'imdl/{version}/doc.json' in 'Select a definition' at the top of Swagger UI.)
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelStage query string true "Stage of RDBMS model to create" Enums(source, planning, target)
// @Param rdbmsModelVersion query string false "RDBMS model version (a cm-beetle 'imdl/vX.Y.Z' tag of v0.1.13 or later, e.g. v0.1.20). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Param Model body CreateRDBMSModelReq true "RDBMS model information"
// @Success 201 {object} object "Successfully created the RDBMS migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Parent (or lineage) model Not Found"
// @Failure 500 {object} model.Response
// @Router /rdbms-model [post]
func CreateRDBMSModel(c echo.Context) error {
	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}

	reqBody := new(rdbmsModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	requestedVer := strings.TrimSpace(c.QueryParam("rdbmsModelVersion"))
	log.Info().Msgf("# CreateRDBMSModel: modelStage=[%s], rdbmsModelVersion=[%s]", modelStage, requestedVer)

	rdbmsVersions, err := getRDBMSModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		return rdbmsModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	resultVer, err := selectModelVersion(requestedVer, rdbmsVersions)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid RDBMS model version: %w", err))
	}
	log.Info().Msgf("RDBMS Model version: %s", resultVer)

	rdbmsModel, status, err := normalizeRDBMSModel(c.Request().Context(), resultVer, modelStage, reqBody.rawRDBMSModel(modelStage))
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	id := uuid.New().String()
	log.Info().Msgf("Generated UUID : [%s]", id)

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categoryRDBMS, modelStage, lineage.MigrationLineage{}, reqBody.ModelLineageReq)
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	createTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	info := RDBMSModelRespInfo{
		Id:              id,
		UserId:          reqBody.UserId,
		NodeId:          reqBody.NodeId,
		IsInitUserModel: reqBody.IsInitUserModel,
		UserModelName:   reqBody.UserModelName,
		UserModelVer:    reqBody.UserModelVer,
		Description:     reqBody.Description,
		RDBMSModelVer:   resultVer,
		CreateTime:      createTime,
		IsRDBMSModel:    true,
		ModelType:       RDBMSModel,
		ModelStage:      modelStage,
		Lineage:         modelLineage,
	}

	var userModel interface{}
	if modelStage == StageTarget {
		userModel = targetRDBMSModelRecord{RDBMSModelRespInfo: info, TargetRDBMSModel: rdbmsModel}
	} else {
		userModel = sourceRDBMSModelRecord{RDBMSModelRespInfo: info, SourceRDBMSModel: rdbmsModel}
	}

	if err := saveRDBMSModelToStore(id, userModel); err != nil {
		return rdbmsModelErrResp(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusCreated, userModel)
}

// UpdateRDBMSModel godoc
// @ID UpdateRDBMSModel
// @Summary Update a specific RDBMS migration user model (source, planning or target)
// @Description Update a specific RDBMS migration user model by ID.
// @Description The model stage is changed to 'modelStage' if it is given, and is preserved otherwise.
// @Description Provide 'sourceRDBMSModel' if the (resulting) stage is source or planning, or 'targetRDBMSModel' if it is target.
// @Description 'createTime' and 'rdbmsModelVersion' of the stored model are preserved, and 'sourceRDBMSModel' (or 'targetRDBMSModel') is validated against the struct of that cm-beetle/imdl tagged version.
// @Description [Lineage] The stored 'lineage' is preserved if neither 'lineage' nor 'parentId' is given. If 'lineage' is given, it replaces the stored one. If 'parentId' is given, the upstream fields are derived from that RDBMS model in the previous stage (source → planning → target).
// @Description The stage cannot be changed while the model is referred to by the lineage of other models (409 Conflict with the referring models in 'list').
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string false "Stage to change the RDBMS model to. If empty, the stored stage is preserved." Enums(source, planning, target)
// @Param Model body CreateRDBMSModelReq true "RDBMS model information to update"
// @Success 200 {object} object "Successfully updated the RDBMS migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The stage cannot be changed: the model is referred to by the lineage of other models"
// @Failure 500 {object} model.Response
// @Router /rdbms-model/{id} [put]
func UpdateRDBMSModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	// 'modelStage' is optional. If it is empty, the stored stage is preserved.
	var requestedStage ModelStage
	if strings.TrimSpace(c.QueryParam("modelStage")) != "" {
		stage, err := parseModelStageParam(c)
		if err != nil {
			return rdbmsModelErrResp(c, http.StatusBadRequest, err)
		}
		requestedStage = stage
	}

	reqBody := new(rdbmsModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	log.Info().Msgf("# UpdateRDBMSModel: id=[%s], modelStage=[%s]", id, requestedStage)

	existing, storedStage, status, err := getRDBMSModelFromStore(id, "")
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	modelStage := storedStage
	if requestedStage != "" && requestedStage != storedStage {
		// The stage of a model referred to by other models is not changed, so as not to break their lineage.
		if referrers := findLineageReferrers(id); len(referrers) > 0 {
			return lineageConflictResp(c, id, "change the stage of", referrers)
		}
		modelStage = requestedStage
		log.Info().Msgf("Changing the model stage of [%s]: [%s] -> [%s]", id, storedStage, requestedStage)
	}

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categoryRDBMS, modelStage, storedLineage(existing, storedStage), reqBody.ModelLineageReq)
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	existingBytes, err := json.Marshal(existing)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to marshal the existing model : [%v]", err))
	}

	// The RDBMS model is validated against the struct of the stored (preserved) cm-beetle/imdl version.
	preservedVer, _ := existing["rdbmsModelVersion"].(string)
	rdbmsVersions, err := getRDBMSModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		return rdbmsModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	var rdbmsModel json.RawMessage
	rawRDBMSModel := reqBody.rawRDBMSModel(modelStage)
	if _, verErr := selectModelVersion(preservedVer, rdbmsVersions); preservedVer != "" && verErr == nil {
		rdbmsModel, status, err = normalizeRDBMSModel(c.Request().Context(), preservedVer, modelStage, rawRDBMSModel)
	} else {
		log.Warn().Msgf("The stored model version [%s] is not a cm-beetle/imdl tag with the RDBMS model. Using the built-in model struct.", preservedVer)
		rdbmsModel, status, err = normalizeRDBMSModelWithCompiledStruct(modelStage, rawRDBMSModel)
	}
	if err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	// The existing model is loaded first so that the fields not given in the request (e.g. 'createTime',
	// 'rdbmsModelVersion') are preserved. If the stage is changed between source/planning and target,
	// the RDBMS model field of the previous stage is not carried over.
	var info *RDBMSModelRespInfo
	var updatedModel interface{}
	if modelStage == StageTarget {
		fullModel := new(targetRDBMSModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return rdbmsModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.TargetRDBMSModel = rdbmsModel
		info, updatedModel = &fullModel.RDBMSModelRespInfo, fullModel
	} else {
		fullModel := new(sourceRDBMSModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return rdbmsModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.SourceRDBMSModel = rdbmsModel
		info, updatedModel = &fullModel.RDBMSModelRespInfo, fullModel
	}
	info.Id = id
	info.UserId = reqBody.UserId
	info.NodeId = reqBody.NodeId
	info.IsInitUserModel = reqBody.IsInitUserModel
	info.UserModelName = reqBody.UserModelName
	info.UserModelVer = reqBody.UserModelVer
	info.Description = reqBody.Description
	info.UpdateTime = updateTime
	info.IsRDBMSModel = true
	info.ModelType = RDBMSModel
	info.ModelStage = modelStage
	info.Lineage = modelLineage

	if err := saveRDBMSModelToStore(id, updatedModel); err != nil {
		return rdbmsModelErrResp(c, http.StatusInternalServerError, err)
	}
	log.Info().Msgf("Successfully updated the model: [%s]", id)

	return c.JSON(http.StatusOK, updatedModel)
}

// DeleteRDBMSModel godoc
// @ID DeleteRDBMSModel
// @Summary Delete a specific RDBMS migration user model (source, planning or target)
// @Description Delete a specific RDBMS migration user model by ID. Use 'modelStage' to specify the stage (source, planning or target) of the model.
// @Description If the model is referred to by the lineage of other models, it is not deleted (409 Conflict with the referring models in 'list') unless 'force' is true.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string true "Stage of the RDBMS model" Enums(source, planning, target)
// @Param force query boolean false "Delete the model even if it is referred to by the lineage of other models" default(false)
// @Success 200 {string} string "Successfully deleted the RDBMS migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The model is referred to by the lineage of other models (listed in 'list')"
// @Failure 500 {object} model.Response
// @Router /rdbms-model/{id} [delete]
func DeleteRDBMSModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return rdbmsModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}
	force, err := parseForceParam(c)
	if err != nil {
		return rdbmsModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# DeleteRDBMSModel: id=[%s], modelStage=[%s], force=[%t]", id, modelStage, force)

	if _, _, status, err := getRDBMSModelFromStore(id, modelStage); err != nil {
		return rdbmsModelErrResp(c, status, err)
	}

	// Verify that no other model refers to this model in its lineage
	if referrers := findLineageReferrers(id); len(referrers) > 0 {
		if !force {
			return lineageConflictResp(c, id, "delete", referrers)
		}
		log.Warn().Msgf("Deleting the model [%s] referred to by the lineage of %d model(s) (force=true)", id, len(referrers))
	}

	lkvstore.Delete(id)
	log.Info().Msgf("Succeeded in Deleting the model : [%s]", id)

	if err := lkvstore.SaveLkvStore(); err != nil {
		return rdbmsModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to save the lkvstore to file : [%v]", err))
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusOK, "Succeeded in Deleting the RDBMS model")
}
