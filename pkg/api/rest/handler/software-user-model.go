package handler

import (    
	"encoding/json"
	"fmt"
	"net/http"
	"errors"
	"strings"
	"github.com/labstack/echo/v4"
	"github.com/google/uuid"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	"github.com/rs/zerolog/log"
	// "github.com/davecgh/go-spew/spew"

	model 			"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-beetle/imdl/lineage"
	softwaremodel 	"github.com/cloud-barista/cm-grasshopper/smdl"
)

// ##############################################################################################
// ### Unified Software Migration User Model (Source + Planning + Target)
// ##############################################################################################

// [Note]
// CreateSoftwareModelReq is a unified request body for creating or updating a source, planning
// or target software migration user model. Only the field matching the 'modelStage' query parameter
// needs to be provided ('sourceSoftwareModel' for source or planning, 'targetSoftwareModel' for target).
type CreateSoftwareModelReq struct {
	UserId              string                                      `json:"userId"`
	NodeId              string                                      `json:"nodeId,omitempty"`
	IsInitUserModel     bool                                        `json:"isInitUserModel"`
	UserModelName       string                                      `json:"userModelName"`
	UserModelVer        string                                      `json:"userModelVersion"`
	Description         string                                      `json:"description"`
	ModelLineageReq
	SourceSoftwareModel *softwaremodel.SourceGroupSoftwareProperty `json:"sourceSoftwareModel,omitempty"`
	TargetSoftwareModel *softwaremodel.TargetGroupSoftwareProperty `json:"targetSoftwareModel,omitempty"`
}

// [Note]
// softwareModelRawReq is the actual request body of CreateSoftwareModel/UpdateSoftwareModel. The software model is kept as
// raw JSON (shadowing the typed fields of CreateSoftwareModelReq) so that it can be validated against the struct of
// the selected cm-grasshopper/smdl tagged version instead of the version compiled into cm-damselfly.
type softwareModelRawReq struct {
	CreateSoftwareModelReq
	SourceSoftwareModel json.RawMessage `json:"sourceSoftwareModel,omitempty"`
	TargetSoftwareModel json.RawMessage `json:"targetSoftwareModel,omitempty"`
}

// softwareModelMeta holds the metadata of a stored software migration user model.
type softwareModelMeta struct {
	Id               string                   `json:"id"`
	UserId           string                   `json:"userId"`
	NodeId           string                   `json:"nodeId,omitempty"`
	IsInitUserModel  bool                     `json:"isInitUserModel"`
	UserModelName    string                   `json:"userModelName"`
	UserModelVer     string                   `json:"userModelVersion"`
	Description      string                   `json:"description"`
	SoftwareModelVer string                   `json:"softwareModelVersion"`
	CreateTime       string                   `json:"createTime"`
	UpdateTime       string                   `json:"updateTime"`
	IsSoftwareModel  bool                     `json:"isSoftwareModel"`
	ModelType        string                   `json:"modelType"`
	ModelStage       ModelStage               `json:"modelStage"`
	Lineage          lineage.MigrationLineage `json:"lineage"`
}

// sourceSoftwareModelRecord and targetSoftwareModelRecord are the stored forms of software models whose
// 'sourceSoftwareModel' / 'targetSoftwareModel' follow the struct of their own smdl version.
// A source or planning model is stored as sourceSoftwareModelRecord, and a target model as targetSoftwareModelRecord.
// (A planning model created before v0.6.6 may be stored as targetSoftwareModelRecord. See isFormerPlanningSoftwareModel.)
type sourceSoftwareModelRecord struct {
	softwareModelMeta
	SourceSoftwareModel json.RawMessage `json:"sourceSoftwareModel"`
}

type targetSoftwareModelRecord struct {
	softwareModelMeta
	TargetSoftwareModel json.RawMessage `json:"targetSoftwareModel"`
}

// rawSoftwareModel returns the raw software model field matching the model stage.
func (r *softwareModelRawReq) rawSoftwareModel(stage ModelStage) json.RawMessage {
	if stage == StageTarget {
		return r.TargetSoftwareModel
	}
	return r.SourceSoftwareModel
}

// isFormerPlanningSoftwareModel reports whether the stored model is a planning model in the former target structure
// ('targetSoftwareModel'), which was used for the planning stage before v0.6.6. It can be read and deleted as it is,
// but it cannot be converted, because the target structure cannot be turned into the source structure.
func isFormerPlanningSoftwareModel(m map[string]interface{}, stage ModelStage) bool {
	return stage == StagePlanning && m[sourceSoftwareModelSchema.FieldName] == nil && m[targetSoftwareModelSchema.FieldName] != nil
}

func softwareModelErrResp(c echo.Context, status int, err error) error {
	if status >= http.StatusInternalServerError {
		log.Error().Msg(err.Error())
	} else {
		log.Warn().Msg(err.Error())
	}
	return c.JSON(status, model.Response{Success: false, Text: err.Error()})
}

// getSoftwareModelFromStore gets the software model with the given ID and verifies that it is a software
// model of the given stage (if 'stage' is empty, only that it is a software model with a valid stage).
// It also returns the stored stage, and on failure, the HTTP status code to respond with.
func getSoftwareModelFromStore(id string, stage ModelStage) (map[string]interface{}, ModelStage, int, error) {
	stored, exists := lkvstore.Get(id)
	if !exists {
		return nil, "", http.StatusNotFound, fmt.Errorf("failed to find the model from db with id: [%s]", id)
	}

	m, ok := stored.(map[string]interface{})
	if !ok {
		return nil, "", http.StatusInternalServerError, fmt.Errorf("internal error: unexpected model data format")
	}

	if isSoftwareModel, ok := m["isSoftwareModel"].(bool); !ok || !isSoftwareModel {
		return nil, "", http.StatusBadRequest, fmt.Errorf("invalid request: the model with id [%s] is not a software model", id)
	}

	storedStage, ok := storedModelStage(m)
	if !ok {
		return nil, "", http.StatusBadRequest, fmt.Errorf("the model with id [%s] has neither a valid 'modelStage' nor 'isTargetModel'", id)
	}
	if stage != "" && storedStage != stage {
		return nil, "", http.StatusBadRequest, fmt.Errorf("model stage mismatch: the software model with id [%s] is a '%s' model, not '%s'",
			id, storedStage, stage)
	}

	return decorateModel(m), storedStage, http.StatusOK, nil
}

// saveSoftwareModelToStore puts the model to the key-value store and saves the store to file.
func saveSoftwareModelToStore(id string, userModel interface{}) error {
	if err := lkvstore.Put(id, userModel); err != nil {
		return fmt.Errorf("failed to put the model to the lkvstore : [%v]", err)
	}
	if err := lkvstore.SaveLkvStore(); err != nil {
		return fmt.Errorf("failed to save the lkvstore to file : [%v]", err)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	return nil
}

// GetSoftwareModels godoc
// @ID GetSoftwareModels
// @Summary Get a list of software migration user models (source, planning or target)
// @Description Get a list of software migration user models. Use 'modelStage' to select the stage (source, planning or target) of the models. Multiple stages can be given by commas (e.g. 'modelStage=source,planning').
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceModelId' to find the models derived from a source model).
// @Description 'category' of each model (infra, software, rdbms or objectStorage) has the same values as the 'category' filter of 'GET /model'.
// @Tags [API] Migration User Models - Software
// @Accept  json
// @Produce  json
// @Param modelStage query []string true "Stages of the models to retrieve. Multiple stages are separated by commas (e.g. source,planning)." Enums(source, planning, target) collectionFormat(csv)
// @Param sourceGroupId   query string false "Filter by 'lineage.sourceGroupId'"
// @Param sourceModelId   query string false "Filter by 'lineage.sourceModelId'"
// @Param planningModelId query string false "Filter by 'lineage.planningModelId'"
// @Param targetModelId   query string false "Filter by 'lineage.targetModelId'"
// @Success 200 {array}  map[string]interface{} "Successfully obtained software migration user models"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 500 {object} model.Response
// @Router /software-model [get]
func GetSoftwareModels(c echo.Context) error {
	modelStages, err := parseModelStagesParam(c, true)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	lineageFilter := parseLineageFilter(c)
	log.Info().Msgf("# GetSoftwareModels: modelStage=%v, lineage=[%+v]", modelStages, lineageFilter)

	modelList, _ := lkvstore.GetWithPrefix("")
	return c.JSON(http.StatusOK, filterModels(modelList, modelStages, categorySoftware, lineageFilter))
}

// GetSoftwareModel godoc
// @ID GetSoftwareModel
// @Summary Get a specific software migration user model (source, planning or target)
// @Description Get a specific software migration user model by ID. Use 'modelStage' to specify the stage (source, planning or target) of the model.
// @Description The 'lineage' of the model refers to its upstream models and itself.
// @Description 'category' of the model (infra, software, rdbms or objectStorage) has the same values as the 'category' filter of 'GET /model'.
// @Tags [API] Migration User Models - Software
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string true "Stage of the software model" Enums(source, planning, target)
// @Success 200 {object} object "Successfully obtained the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /software-model/{id} [get]
func GetSoftwareModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# GetSoftwareModel: id=[%s], modelStage=[%s]", id, modelStage)

	m, _, status, err := getSoftwareModelFromStore(id, modelStage)
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	return c.JSON(http.StatusOK, m)
}

// ConvertSoftwareModel godoc
// @ID ConvertSoftwareModel
// @Summary Convert a stored software migration user model to a newer model version
// @Description Get a stored software migration user model converted to the struct of the cm-grasshopper/smdl tagged version given by 'modelVersion'. The stored model is not changed.
// @Description Only the conversion to the same or a newer version is allowed, and a model whose stored version is not a cm-grasshopper/smdl tag cannot be converted.
// @Description The response has the fields of 'GET /software-model/{id}', with 'softwareModelVersion' and 'sourceSoftwareModel' (or 'targetSoftwareModel') converted, and 'conversion' added: 'originVersion' (the stored version), 'currentVersion' (the version of the returned model), 'converted' (false if the stored version is already the requested version), 'droppedFields' (the paths of the fields removed because the requested version does not have them) and 'warnings' (what could not be converted as it was).
// @Description If a field of the stored model has a value which does not match the type of that field in the requested version (e.g. the free-form 'resources' of Kubernetes before v0.1.4), the model is not converted (400).
// @Description A planning model stored in the former target structure ('targetSoftwareModel', before v0.6.6) cannot be converted (400). Update it with 'sourceSoftwareModel' first.
// @Tags [API] Migration User Models - Software
// @Accept  json
// @Produce  json
// @Param id           path  string true  "Model ID"
// @Param modelStage   query string true  "Stage of the software model" Enums(source, planning, target)
// @Param modelVersion query string false "Model version to convert to (a cm-grasshopper 'smdl/vX.Y.Z' tag, e.g. v0.1.4). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Success 200 {object} object "Successfully obtained the converted software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter, or the model cannot be converted to the version"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /software-model/{id}/convert [get]
func ConvertSoftwareModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# ConvertSoftwareModel: id=[%s], modelStage=[%s], modelVersion=[%s]", id, modelStage, c.QueryParam("modelVersion"))

	m, storedStage, status, err := getSoftwareModelFromStore(id, modelStage)
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}
	if isFormerPlanningSoftwareModel(m, storedStage) {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("the planning model with id [%s] is stored in the former target structure ('targetSoftwareModel') and cannot be converted. "+
			"Update it with 'sourceSoftwareModel' (PUT /software-model/{id}) or create a new planning model", id))
	}

	softwareVersions, err := getSoftwareModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-grasshopper/smdl model versions: %w", err)
		return softwareModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	currentVersion, err := selectModelVersion(c.QueryParam("modelVersion"), softwareVersions)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid software model version: %w", err))
	}

	originVersion, _ := m["softwareModelVersion"].(string)
	if err := validateSoftwareModelConversion(originVersion, currentVersion, softwareVersions); err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}

	converted := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		converted[k] = v
	}

	if originVersion == currentVersion {
		converted["conversion"] = newSoftwareModelConversion(originVersion, currentVersion, false)
		return c.JSON(http.StatusOK, converted)
	}

	schema := softwareModelSchemaOf(modelStage)
	if m[schema.FieldName] == nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("the stored model with id [%s] has no '%s' to convert", id, schema.FieldName))
	}
	rawSoftwareModel, err := json.Marshal(m[schema.FieldName])
	if err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to read the stored '%s' : [%v]", schema.FieldName, err))
	}

	pkg, err := modelschema.LoadPackage(c.Request().Context(), softwareModelSchemaSource, currentVersion, "")
	if err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to load the cm-grasshopper/smdl %s model : [%v]", currentVersion, err))
	}
	if !pkg.HasType(schema.RootType) {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("'%s' struct does not exist in cm-grasshopper/smdl %s. Use another version", schema.RootType, currentVersion))
	}

	softwareModel, conversion, err := convertSoftwareModel(pkg, modelStage, originVersion, currentVersion, rawSoftwareModel)
	if err != nil {
		newErr := fmt.Errorf("failed to convert the model with id [%s] from %s to %s : [%v]", id, originVersion, currentVersion, err)
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return softwareModelErrResp(c, http.StatusBadRequest, newErr)
		}
		return softwareModelErrResp(c, http.StatusInternalServerError, newErr)
	}
	log.Info().Msgf("Converted the model [%s] from %s to %s (dropped fields: %d)", id, originVersion, currentVersion, len(conversion.DroppedFields))

	converted["softwareModelVersion"] = currentVersion
	converted[schema.FieldName] = json.RawMessage(softwareModel)
	converted["conversion"] = conversion
	return c.JSON(http.StatusOK, converted)
}

// CreateSoftwareModel godoc
// @ID CreateSoftwareModel
// @Summary Create a new software migration user model (source, planning or target)
// @Description Create a new software migration user model. Use 'modelStage' to select the stage (source, planning or target) of the model.
// @Description Provide 'sourceSoftwareModel' for a source or planning model (modelStage=source or planning), or 'targetSoftwareModel' for a target model (modelStage=target).
// @Description The 'sourceSoftwareModel' (or 'targetSoftwareModel') is validated against the 'SourceGroupSoftwareProperty' (or 'TargetGroupSoftwareProperty') struct of the cm-grasshopper/smdl tagged version given by 'softwareModelVersion', and is stored in that struct's form.
// @Description 'nodeId' is an optional identifier of the node the model is associated with.
// @Description [Lineage] 'parentId' is the ID of the software model this model is derived from, which must be in the previous stage (source → planning → target): a source model for a planning model, and a planning model for a target model. The upstream fields of 'lineage' are derived from the parent.
// @Description 'lineage' can also be given explicitly (e.g. 'sourceGroupId'). The field of the model's own stage (e.g. 'planningModelId' of a planning model) is set to the ID of the created model by the server.
// @Description (The body schema below shows the struct of the cm-grasshopper/smdl version built into cm-damselfly. To see the struct of another version, choose 'smdl/{version}/doc.json' in 'Select a definition' at the top of Swagger UI.)
// @Tags [API] Migration User Models - Software
// @Accept  json
// @Produce  json
// @Param modelStage query string true "Stage of software model to create" Enums(source, planning, target)
// @Param softwareModelVersion query string false "Software model version (a cm-grasshopper 'smdl/vX.Y.Z' tag, e.g. v0.1.3). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Param Model body CreateSoftwareModelReq true "Software model information"
// @Success 201 {object} object "Successfully created the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Parent (or lineage) model Not Found"
// @Failure 500 {object} model.Response
// @Router /software-model [post]
func CreateSoftwareModel(c echo.Context) error {
	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}

	reqBody := new(softwareModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	requestedVer := strings.TrimSpace(c.QueryParam("softwareModelVersion"))
	log.Info().Msgf("# CreateSoftwareModel: modelStage=[%s], softwareModelVersion=[%s]", modelStage, requestedVer)

	softwareVersions, err := getSoftwareModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-grasshopper/smdl model versions: %w", err)
		return softwareModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	resultVer, err := selectModelVersion(requestedVer, softwareVersions)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid software model version: %w", err))
	}
	log.Info().Msgf("Software Model version: %s", resultVer)

	softwareModel, status, err := normalizeSoftwareModel(c.Request().Context(), resultVer, modelStage, reqBody.rawSoftwareModel(modelStage))
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	id := uuid.New().String()
	log.Info().Msgf("Generated UUID : [%s]", id)

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categorySoftware, modelStage, lineage.MigrationLineage{}, reqBody.ModelLineageReq)
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	createTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	info := softwareModelMeta{
		Id:               id,
		UserId:           reqBody.UserId,
		NodeId:           reqBody.NodeId,
		IsInitUserModel:  reqBody.IsInitUserModel,
		UserModelName:    reqBody.UserModelName,
		UserModelVer:     reqBody.UserModelVer,
		Description:      reqBody.Description,
		SoftwareModelVer: resultVer,
		CreateTime:       createTime,
		IsSoftwareModel:  true,
		ModelType:        SWModel,
		ModelStage:       modelStage,
		Lineage:          modelLineage,
	}

	var userModel interface{}
	if modelStage == StageTarget {
		userModel = targetSoftwareModelRecord{softwareModelMeta: info, TargetSoftwareModel: softwareModel}
	} else {
		userModel = sourceSoftwareModelRecord{softwareModelMeta: info, SourceSoftwareModel: softwareModel}
	}

	if err := saveSoftwareModelToStore(id, userModel); err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, err)
	}

	resp, err := decorateRecord(userModel)
	if err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// UpdateSoftwareModel godoc
// @ID UpdateSoftwareModel
// @Summary Update a specific software migration user model (source, planning or target)
// @Description Update a specific software migration user model by ID.
// @Description The model stage is changed to 'modelStage' if it is given, and is preserved otherwise.
// @Description Provide 'sourceSoftwareModel' if the (resulting) stage is source or planning, or 'targetSoftwareModel' if it is target.
// @Description 'createTime' and 'softwareModelVersion' of the stored model are preserved, and 'sourceSoftwareModel' (or 'targetSoftwareModel') is validated against the struct of that cm-grasshopper/smdl tagged version.
// @Description [Lineage] The stored 'lineage' is preserved if neither 'lineage' nor 'parentId' is given. If 'lineage' is given, it replaces the stored one. If 'parentId' is given, the upstream fields are derived from that model in the previous stage (source → planning → target).
// @Description The stage cannot be changed while the model is referred to by the lineage of other models (409 Conflict with the referring models in 'list').
// @Tags [API] Migration User Models - Software
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string false "Stage to change the software model to. If empty, the stored stage is preserved." Enums(source, planning, target)
// @Param Model body CreateSoftwareModelReq true "Software model information to update"
// @Success 200 {object} object "Successfully updated the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The stage cannot be changed: the model is referred to by the lineage of other models"
// @Failure 500 {object} model.Response
// @Router /software-model/{id} [put]
func UpdateSoftwareModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	// 'modelStage' is optional. If it is empty, the stored stage is preserved.
	var requestedStage ModelStage
	if strings.TrimSpace(c.QueryParam("modelStage")) != "" {
		stage, err := parseModelStageParam(c)
		if err != nil {
			return softwareModelErrResp(c, http.StatusBadRequest, err)
		}
		requestedStage = stage
	}

	reqBody := new(softwareModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	log.Info().Msgf("# UpdateSoftwareModel: id=[%s], modelStage=[%s]", id, requestedStage)

	existing, storedStage, status, err := getSoftwareModelFromStore(id, "")
	if err != nil {
		return softwareModelErrResp(c, status, err)
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

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categorySoftware, modelStage, storedLineage(existing, storedStage), reqBody.ModelLineageReq)
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	existingBytes, err := json.Marshal(existing)
	if err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to marshal the existing model : [%v]", err))
	}

	// The software model is validated against the struct of the stored (preserved) smdl version.
	preservedVer, _ := existing["softwareModelVersion"].(string)
	softwareVersions, err := getSoftwareModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-grasshopper/smdl model versions: %w", err)
		return softwareModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	var softwareModel json.RawMessage
	rawSoftwareModel := reqBody.rawSoftwareModel(modelStage)
	if _, verErr := selectModelVersion(preservedVer, softwareVersions); preservedVer != "" && verErr == nil {
		softwareModel, status, err = normalizeSoftwareModel(c.Request().Context(), preservedVer, modelStage, rawSoftwareModel)
	} else {
		log.Warn().Msgf("The stored model version [%s] is not a cm-grasshopper/smdl tag. Using the built-in model struct.", preservedVer)
		softwareModel, status, err = normalizeSoftwareModelWithCompiledStruct(modelStage, rawSoftwareModel)
	}
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	// The existing model is loaded first so that the fields not given in the request
	// (e.g. 'createTime', 'softwareModelVersion') are preserved. If the stage is changed between
	// source/planning and target, the software model field of the previous stage is not carried over.
	var info *softwareModelMeta
	var updatedModel interface{}
	if modelStage == StageTarget {
		fullModel := new(targetSoftwareModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.TargetSoftwareModel = softwareModel
		info, updatedModel = &fullModel.softwareModelMeta, fullModel
	} else {
		fullModel := new(sourceSoftwareModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.SourceSoftwareModel = softwareModel
		info, updatedModel = &fullModel.softwareModelMeta, fullModel
	}
	info.Id = id
	info.UserId = reqBody.UserId
	info.NodeId = reqBody.NodeId
	info.IsInitUserModel = reqBody.IsInitUserModel
	info.UserModelName = reqBody.UserModelName
	info.UserModelVer = reqBody.UserModelVer
	info.Description = reqBody.Description
	info.UpdateTime = updateTime
	info.IsSoftwareModel = true
	info.ModelType = SWModel
	info.ModelStage = modelStage
	info.Lineage = modelLineage

	if err := saveSoftwareModelToStore(id, updatedModel); err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, err)
	}
	log.Info().Msgf("Successfully updated the model: [%s]", id)

	resp, err := decorateRecord(updatedModel)
	if err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// DeleteSoftwareModel godoc
// @ID DeleteSoftwareModel
// @Summary Delete a specific software migration user model (source, planning or target)
// @Description Delete a specific software migration user model by ID. Use 'modelStage' to specify the stage (source, planning or target) of the model.
// @Description If the model is referred to by the lineage of other models, it is not deleted (409 Conflict with the referring models in 'list') unless 'force' is true.
// @Tags [API] Migration User Models - Software
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string true "Stage of the software model" Enums(source, planning, target)
// @Param force query boolean false "Delete the model even if it is referred to by the lineage of other models" default(false)
// @Success 200 {string} string "Successfully deleted the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The model is referred to by the lineage of other models (listed in 'list')"
// @Failure 500 {object} model.Response
// @Router /software-model/{id} [delete]
func DeleteSoftwareModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	force, err := parseForceParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# DeleteSoftwareModel: id=[%s], modelStage=[%s], force=[%t]", id, modelStage, force)

	if _, _, status, err := getSoftwareModelFromStore(id, modelStage); err != nil {
		return softwareModelErrResp(c, status, err)
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
		return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to save the lkvstore to file : [%v]", err))
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusOK, "Succeeded in Deleting the software model")
}
