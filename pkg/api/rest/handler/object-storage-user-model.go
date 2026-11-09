package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloud-barista/cm-beetle/imdl/lineage"
	storagemodel "github.com/cloud-barista/cm-beetle/imdl/storage-model"
	model "github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	"github.com/google/uuid"
	echo "github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ##############################################################################################
// ### Object Storage Migration User Model (Source + Planning + Target)
// ##############################################################################################

// [Note]
// An object storage migration user model follows the struct of the 'storage-model' package of a cm-beetle/imdl tagged
// version (v0.1.3 or later), which is given by 'objectStorageModelVersion':
//   - A source or planning model has 'sourceObjectStorageModel' ('SourceObjectStorage' struct, the input of the
//     cm-beetle object storage recommendation).
//   - A target model has 'targetObjectStorageModel' ('RecommendedObjectStorage' struct, the input of the cm-beetle
//     object storage migration).
// The lineage links the object storage models only (category 'objectStorage').

// CreateObjectStorageModelReq is the request body for creating or updating an object storage migration user model.
// Only the field matching the 'modelStage' query parameter needs to be provided
// ('sourceObjectStorageModel' for source or planning, 'targetObjectStorageModel' for target).
type CreateObjectStorageModelReq struct {
	UserId          string `json:"userId"`
	NodeId          string `json:"nodeId,omitempty"`
	IsInitUserModel bool   `json:"isInitUserModel"`
	UserModelName   string `json:"userModelName"`
	UserModelVer    string `json:"userModelVersion"`
	Description     string `json:"description"`
	ModelLineageReq
	SourceObjectStorageModel *storagemodel.SourceObjectStorage      `json:"sourceObjectStorageModel,omitempty"`
	TargetObjectStorageModel *storagemodel.RecommendedObjectStorage `json:"targetObjectStorageModel,omitempty"`
}

// [Note]
// objectStorageModelRawReq is the actual request body of CreateObjectStorageModel/UpdateObjectStorageModel. The object storage model
// is kept as raw JSON (shadowing the typed fields of CreateObjectStorageModelReq) so that it can be validated against the
// struct of the selected cm-beetle/imdl tagged version instead of the version compiled into cm-damselfly.
type objectStorageModelRawReq struct {
	CreateObjectStorageModelReq
	SourceObjectStorageModel json.RawMessage `json:"sourceObjectStorageModel,omitempty"`
	TargetObjectStorageModel json.RawMessage `json:"targetObjectStorageModel,omitempty"`
}

// rawObjectStorageModel returns the raw object storage model field matching the model stage.
func (r *objectStorageModelRawReq) rawObjectStorageModel(stage ModelStage) json.RawMessage {
	if stage == StageTarget {
		return r.TargetObjectStorageModel
	}
	return r.SourceObjectStorageModel
}

// ObjectStorageModelRespInfo holds the metadata of a stored object storage migration user model.
type ObjectStorageModelRespInfo struct {
	Id                    string                   `json:"id"`
	UserId                string                   `json:"userId"`
	NodeId                string                   `json:"nodeId,omitempty"`
	IsInitUserModel       bool                     `json:"isInitUserModel"`
	UserModelName         string                   `json:"userModelName"`
	UserModelVer          string                   `json:"userModelVersion"`
	Description           string                   `json:"description"`
	ObjectStorageModelVer string                   `json:"objectStorageModelVersion"`
	CreateTime            string                   `json:"createTime"`
	UpdateTime            string                   `json:"updateTime"`
	IsObjectStorageModel  bool                     `json:"isObjectStorageModel"`
	ModelType             string                   `json:"modelType"`
	ModelStage            ModelStage               `json:"modelStage"`
	Lineage               lineage.MigrationLineage `json:"lineage"`
}

// sourceObjectStorageModelRecord and targetObjectStorageModelRecord are the stored forms of object storage models whose
// 'sourceObjectStorageModel' / 'targetObjectStorageModel' follow the struct of their own cm-beetle/imdl version.
// A source or planning model is stored as sourceObjectStorageModelRecord, and a target model as targetObjectStorageModelRecord.
type sourceObjectStorageModelRecord struct {
	ObjectStorageModelRespInfo
	SourceObjectStorageModel json.RawMessage `json:"sourceObjectStorageModel"`
}

type targetObjectStorageModelRecord struct {
	ObjectStorageModelRespInfo
	TargetObjectStorageModel json.RawMessage `json:"targetObjectStorageModel"`
}

func objectStorageModelErrResp(c echo.Context, status int, err error) error {
	if status >= http.StatusInternalServerError {
		log.Error().Msg(err.Error())
	} else {
		log.Warn().Msg(err.Error())
	}
	return c.JSON(status, model.Response{Success: false, Text: err.Error()})
}

// isStoredObjectStorageModel reports whether the stored model is an object storage model.
func isStoredObjectStorageModel(m map[string]interface{}) bool {
	isObjectStorageModel, ok := m["isObjectStorageModel"].(bool)
	return ok && isObjectStorageModel
}

// getObjectStorageModelFromStore gets the object storage model with the given ID and verifies that it is an object storage
// model of the given stage (if 'stage' is empty, only that it is an object storage model with a valid stage).
// It also returns the stored stage, and on failure, the HTTP status code to respond with.
func getObjectStorageModelFromStore(id string, stage ModelStage) (map[string]interface{}, ModelStage, int, error) {
	stored, exists := lkvstore.Get(id)
	if !exists {
		return nil, "", http.StatusNotFound, fmt.Errorf("failed to find the model from db with id: [%s]", id)
	}

	m, ok := stored.(map[string]interface{})
	if !ok {
		return nil, "", http.StatusInternalServerError, fmt.Errorf("internal error: unexpected model data format")
	}

	if !isStoredObjectStorageModel(m) {
		return nil, "", http.StatusBadRequest, fmt.Errorf("invalid request: the model with id [%s] is not an object storage model", id)
	}

	storedStage, ok := storedModelStage(m)
	if !ok {
		return nil, "", http.StatusBadRequest, fmt.Errorf("the model with id [%s] has no valid 'modelStage'", id)
	}
	if stage != "" && storedStage != stage {
		return nil, "", http.StatusBadRequest, fmt.Errorf("model stage mismatch: the object storage model with id [%s] is a '%s' model, not '%s'",
			id, storedStage, stage)
	}

	return withLineage(withModelStage(m, storedStage), storedStage), storedStage, http.StatusOK, nil
}

// saveObjectStorageModelToStore puts the model to the key-value store and saves the store to file.
func saveObjectStorageModelToStore(id string, userModel interface{}) error {
	if err := lkvstore.Put(id, userModel); err != nil {
		return fmt.Errorf("failed to put the model to the lkvstore : [%v]", err)
	}
	if err := lkvstore.SaveLkvStore(); err != nil {
		return fmt.Errorf("failed to save the lkvstore to file : [%v]", err)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	return nil
}

// GetObjectStorageModels godoc
// @ID GetObjectStorageModels
// @Summary Get a list of object storage migration user models (source, planning or target)
// @Description Get a list of object storage migration user models. Use 'modelStage' to select the stage (source, planning or target) of the models.
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceModelId' to find the models derived from a source model).
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelStage query string true "Stage of object storage model to retrieve" Enums(source, planning, target)
// @Param sourceGroupId   query string false "Filter by 'lineage.sourceGroupId'"
// @Param sourceModelId   query string false "Filter by 'lineage.sourceModelId'"
// @Param planningModelId query string false "Filter by 'lineage.planningModelId'"
// @Param targetModelId   query string false "Filter by 'lineage.targetModelId'"
// @Success 200 {array}  map[string]interface{} "Successfully obtained object storage migration user models"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 500 {object} model.Response
// @Router /object-storage-model [get]
func GetObjectStorageModels(c echo.Context) error {
	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}
	lineageFilter := parseLineageFilter(c)
	log.Info().Msgf("# GetObjectStorageModels: modelStage=[%s], lineage=[%+v]", modelStage, lineageFilter)

	modelList, _ := lkvstore.GetWithPrefix("")
	return c.JSON(http.StatusOK, filterModels(modelList, modelStage, categoryObjectStorage, lineageFilter))
}

// GetObjectStorageModel godoc
// @ID GetObjectStorageModel
// @Summary Get a specific object storage migration user model (source, planning or target)
// @Description Get a specific object storage migration user model by ID. Use 'modelStage' to specify the stage (source, planning or target) of the model.
// @Description The 'lineage' of the model refers to its upstream models and itself.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string true "Stage of the object storage model" Enums(source, planning, target)
// @Success 200 {object} object "Successfully obtained the object storage migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /object-storage-model/{id} [get]
func GetObjectStorageModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# GetObjectStorageModel: id=[%s], modelStage=[%s]", id, modelStage)

	m, _, status, err := getObjectStorageModelFromStore(id, modelStage)
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
	}

	return c.JSON(http.StatusOK, m)
}

// ConvertObjectStorageModel godoc
// @ID ConvertObjectStorageModel
// @Summary Convert a stored object storage migration user model to a newer model version
// @Description Get a stored object storage migration user model converted to the struct of the cm-beetle/imdl tagged version given by 'objectStorageModelVersion'. The stored model is not changed.
// @Description Only the conversion to the same or a newer version is allowed, and a model whose stored version is not a cm-beetle/imdl tag with the object storage model (v0.1.3 or later) cannot be converted.
// @Description The response has the fields of 'GET /object-storage-model/{id}', with 'objectStorageModelVersion' and 'sourceObjectStorageModel' (or 'targetObjectStorageModel') converted, and 'conversion' added: 'originVersion' (the stored version), 'currentVersion' (the version of the returned model), 'converted' (false if the stored version is already the requested version), 'droppedFields' (the paths of the fields removed because the requested version does not have them) and 'warnings' (what could not be converted as it was).
// @Description [Migrations] v0.1.6: 'nameSeed' of 'targetObjectStorageModel' was removed (reported in 'droppedFields', and in 'warnings' if it has a value). v0.1.11: 'sourceObjectStorageModel', which described a single bucket, is converted to a group of buckets: the bucket fields are moved into 'sourceObjectStorages[0]'.
// @Description If a field of the stored model has a value which does not match the type of that field in the requested version, the model is not converted (400).
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id                        path  string true  "Model ID"
// @Param modelStage                query string true  "Stage of the object storage model" Enums(source, planning, target)
// @Param objectStorageModelVersion query string false "Model version to convert to (a cm-beetle 'imdl/vX.Y.Z' tag of v0.1.3 or later, e.g. v0.1.20). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Success 200 {object} object "Successfully obtained the converted object storage migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter, or the model cannot be converted to the version"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /object-storage-model/{id}/convert [get]
func ConvertObjectStorageModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# ConvertObjectStorageModel: id=[%s], modelStage=[%s], objectStorageModelVersion=[%s]", id, modelStage, c.QueryParam("objectStorageModelVersion"))

	m, _, status, err := getObjectStorageModelFromStore(id, modelStage)
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
	}

	storageVersions, err := getObjectStorageModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		return objectStorageModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	currentVersion, err := selectModelVersion(c.QueryParam("objectStorageModelVersion"), storageVersions)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid object storage model version: %w", err))
	}

	originVersion, _ := m["objectStorageModelVersion"].(string)
	if err := validateObjectStorageModelConversion(originVersion, currentVersion, storageVersions); err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}

	converted := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		converted[k] = v
	}

	if originVersion == currentVersion {
		converted["conversion"] = newObjectStorageModelConversion(originVersion, currentVersion, false)
		return c.JSON(http.StatusOK, converted)
	}

	schema := objectStorageModelSchemaOf(modelStage)
	if m[schema.FieldName] == nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("the stored model with id [%s] has no '%s' to convert", id, schema.FieldName))
	}
	rawStorageModel, err := json.Marshal(m[schema.FieldName])
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to read the stored '%s' : [%v]", schema.FieldName, err))
	}

	pkg, err := modelschema.LoadPackage(c.Request().Context(), infraModelSchemaSource, currentVersion, objectStorageModelPkgDir)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to load the cm-beetle/imdl %s '%s' model : [%v]", currentVersion, objectStorageModelPkgDir, err))
	}
	if !pkg.HasType(schema.RootType) {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("'%s' struct does not exist in cm-beetle/imdl %s '%s'. Use another version", schema.RootType, currentVersion, objectStorageModelPkgDir))
	}

	storageModel, conversion, err := convertObjectStorageModel(pkg, modelStage, originVersion, currentVersion, rawStorageModel)
	if err != nil {
		newErr := fmt.Errorf("failed to convert the model with id [%s] from %s to %s : [%v]", id, originVersion, currentVersion, err)
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return objectStorageModelErrResp(c, http.StatusBadRequest, newErr)
		}
		return objectStorageModelErrResp(c, http.StatusInternalServerError, newErr)
	}
	log.Info().Msgf("Converted the model [%s] from %s to %s (dropped fields: %d, warnings: %d)", id, originVersion, currentVersion, len(conversion.DroppedFields), len(conversion.Warnings))

	converted["objectStorageModelVersion"] = currentVersion
	converted[schema.FieldName] = storageModel
	converted["conversion"] = conversion
	return c.JSON(http.StatusOK, converted)
}

// CreateObjectStorageModel godoc
// @ID CreateObjectStorageModel
// @Summary Create a new object storage migration user model (source, planning or target)
// @Description Create a new object storage migration user model. Use 'modelStage' to select the stage (source, planning or target) of the model.
// @Description Provide 'sourceObjectStorageModel' for a source or planning model (modelStage=source or planning), or 'targetObjectStorageModel' for a target model (modelStage=target).
// @Description The 'sourceObjectStorageModel' (or 'targetObjectStorageModel') is validated against the 'SourceObjectStorage' (or 'RecommendedObjectStorage') struct of the cm-beetle/imdl tagged version given by 'objectStorageModelVersion', and is stored in that struct's form.
// @Description 'nodeId' is an optional identifier of the node the model is associated with.
// @Description [Lineage] 'parentId' is the ID of the object storage model this model is derived from, which must be in the previous stage (source → planning → target): a source model for a planning model, and a planning model for a target model. The upstream fields of 'lineage' are derived from the parent.
// @Description 'lineage' can also be given explicitly (e.g. 'sourceGroupId'). The field of the model's own stage (e.g. 'planningModelId' of a planning model) is set to the ID of the created model by the server.
// @Description (The body schema below shows the struct of the cm-beetle/imdl version built into cm-damselfly. To see the struct of another version, choose 'imdl/{version}/doc.json' in 'Select a definition' at the top of Swagger UI.)
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelStage query string true "Stage of object storage model to create" Enums(source, planning, target)
// @Param objectStorageModelVersion query string false "Object storage model version (a cm-beetle 'imdl/vX.Y.Z' tag of v0.1.3 or later, e.g. v0.1.20). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Param Model body CreateObjectStorageModelReq true "Object storage model information"
// @Success 201 {object} object "Successfully created the object storage migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Parent (or lineage) model Not Found"
// @Failure 500 {object} model.Response
// @Router /object-storage-model [post]
func CreateObjectStorageModel(c echo.Context) error {
	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}

	reqBody := new(objectStorageModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	requestedVer := strings.TrimSpace(c.QueryParam("objectStorageModelVersion"))
	log.Info().Msgf("# CreateObjectStorageModel: modelStage=[%s], objectStorageModelVersion=[%s]", modelStage, requestedVer)

	storageVersions, err := getObjectStorageModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		return objectStorageModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	resultVer, err := selectModelVersion(requestedVer, storageVersions)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid object storage model version: %w", err))
	}
	log.Info().Msgf("Object Storage Model version: %s", resultVer)

	storageModel, status, err := normalizeObjectStorageModel(c.Request().Context(), resultVer, modelStage, reqBody.rawObjectStorageModel(modelStage))
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
	}

	id := uuid.New().String()
	log.Info().Msgf("Generated UUID : [%s]", id)

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categoryObjectStorage, modelStage, lineage.MigrationLineage{}, reqBody.ModelLineageReq)
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
	}

	createTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	info := ObjectStorageModelRespInfo{
		Id:                    id,
		UserId:                reqBody.UserId,
		NodeId:                reqBody.NodeId,
		IsInitUserModel:       reqBody.IsInitUserModel,
		UserModelName:         reqBody.UserModelName,
		UserModelVer:          reqBody.UserModelVer,
		Description:           reqBody.Description,
		ObjectStorageModelVer: resultVer,
		CreateTime:            createTime,
		IsObjectStorageModel:  true,
		ModelType:             ObjectStorageModel,
		ModelStage:            modelStage,
		Lineage:               modelLineage,
	}

	var userModel interface{}
	if modelStage == StageTarget {
		userModel = targetObjectStorageModelRecord{ObjectStorageModelRespInfo: info, TargetObjectStorageModel: storageModel}
	} else {
		userModel = sourceObjectStorageModelRecord{ObjectStorageModelRespInfo: info, SourceObjectStorageModel: storageModel}
	}

	if err := saveObjectStorageModelToStore(id, userModel); err != nil {
		return objectStorageModelErrResp(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusCreated, userModel)
}

// UpdateObjectStorageModel godoc
// @ID UpdateObjectStorageModel
// @Summary Update a specific object storage migration user model (source, planning or target)
// @Description Update a specific object storage migration user model by ID.
// @Description The model stage is changed to 'modelStage' if it is given, and is preserved otherwise.
// @Description Provide 'sourceObjectStorageModel' if the (resulting) stage is source or planning, or 'targetObjectStorageModel' if it is target.
// @Description 'createTime' and 'objectStorageModelVersion' of the stored model are preserved, and 'sourceObjectStorageModel' (or 'targetObjectStorageModel') is validated against the struct of that cm-beetle/imdl tagged version.
// @Description [Lineage] The stored 'lineage' is preserved if neither 'lineage' nor 'parentId' is given. If 'lineage' is given, it replaces the stored one. If 'parentId' is given, the upstream fields are derived from that object storage model in the previous stage (source → planning → target).
// @Description The stage cannot be changed while the model is referred to by the lineage of other models (409 Conflict with the referring models in 'list').
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string false "Stage to change the object storage model to. If empty, the stored stage is preserved." Enums(source, planning, target)
// @Param Model body CreateObjectStorageModelReq true "Object storage model information to update"
// @Success 200 {object} object "Successfully updated the object storage migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The stage cannot be changed: the model is referred to by the lineage of other models"
// @Failure 500 {object} model.Response
// @Router /object-storage-model/{id} [put]
func UpdateObjectStorageModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	// 'modelStage' is optional. If it is empty, the stored stage is preserved.
	var requestedStage ModelStage
	if strings.TrimSpace(c.QueryParam("modelStage")) != "" {
		stage, err := parseModelStageParam(c)
		if err != nil {
			return objectStorageModelErrResp(c, http.StatusBadRequest, err)
		}
		requestedStage = stage
	}

	reqBody := new(objectStorageModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	log.Info().Msgf("# UpdateObjectStorageModel: id=[%s], modelStage=[%s]", id, requestedStage)

	existing, storedStage, status, err := getObjectStorageModelFromStore(id, "")
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
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

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categoryObjectStorage, modelStage, storedLineage(existing, storedStage), reqBody.ModelLineageReq)
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
	}

	existingBytes, err := json.Marshal(existing)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to marshal the existing model : [%v]", err))
	}

	// The object storage model is validated against the struct of the stored (preserved) cm-beetle/imdl version.
	preservedVer, _ := existing["objectStorageModelVersion"].(string)
	storageVersions, err := getObjectStorageModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		return objectStorageModelErrResp(c, http.StatusInternalServerError, newErr)
	}

	var storageModel json.RawMessage
	rawStorageModel := reqBody.rawObjectStorageModel(modelStage)
	if _, verErr := selectModelVersion(preservedVer, storageVersions); preservedVer != "" && verErr == nil {
		storageModel, status, err = normalizeObjectStorageModel(c.Request().Context(), preservedVer, modelStage, rawStorageModel)
	} else {
		log.Warn().Msgf("The stored model version [%s] is not a cm-beetle/imdl tag with the object storage model. Using the built-in model struct.", preservedVer)
		storageModel, status, err = normalizeObjectStorageModelWithCompiledStruct(modelStage, rawStorageModel)
	}
	if err != nil {
		return objectStorageModelErrResp(c, status, err)
	}

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	// The existing model is loaded first so that the fields not given in the request (e.g. 'createTime',
	// 'objectStorageModelVersion') are preserved. If the stage is changed between source/planning and target,
	// the object storage model field of the previous stage is not carried over.
	var info *ObjectStorageModelRespInfo
	var updatedModel interface{}
	if modelStage == StageTarget {
		fullModel := new(targetObjectStorageModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return objectStorageModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.TargetObjectStorageModel = storageModel
		info, updatedModel = &fullModel.ObjectStorageModelRespInfo, fullModel
	} else {
		fullModel := new(sourceObjectStorageModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return objectStorageModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.SourceObjectStorageModel = storageModel
		info, updatedModel = &fullModel.ObjectStorageModelRespInfo, fullModel
	}
	info.Id = id
	info.UserId = reqBody.UserId
	info.NodeId = reqBody.NodeId
	info.IsInitUserModel = reqBody.IsInitUserModel
	info.UserModelName = reqBody.UserModelName
	info.UserModelVer = reqBody.UserModelVer
	info.Description = reqBody.Description
	info.UpdateTime = updateTime
	info.IsObjectStorageModel = true
	info.ModelType = ObjectStorageModel
	info.ModelStage = modelStage
	info.Lineage = modelLineage

	if err := saveObjectStorageModelToStore(id, updatedModel); err != nil {
		return objectStorageModelErrResp(c, http.StatusInternalServerError, err)
	}
	log.Info().Msgf("Successfully updated the model: [%s]", id)

	return c.JSON(http.StatusOK, updatedModel)
}

// DeleteObjectStorageModel godoc
// @ID DeleteObjectStorageModel
// @Summary Delete a specific object storage migration user model (source, planning or target)
// @Description Delete a specific object storage migration user model by ID. Use 'modelStage' to specify the stage (source, planning or target) of the model.
// @Description If the model is referred to by the lineage of other models, it is not deleted (409 Conflict with the referring models in 'list') unless 'force' is true.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param modelStage query string true "Stage of the object storage model" Enums(source, planning, target)
// @Param force query boolean false "Delete the model even if it is referred to by the lineage of other models" default(false)
// @Success 200 {string} string "Successfully deleted the object storage migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The model is referred to by the lineage of other models (listed in 'list')"
// @Failure 500 {object} model.Response
// @Router /object-storage-model/{id} [delete]
func DeleteObjectStorageModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return objectStorageModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}
	force, err := parseForceParam(c)
	if err != nil {
		return objectStorageModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# DeleteObjectStorageModel: id=[%s], modelStage=[%s], force=[%t]", id, modelStage, force)

	if _, _, status, err := getObjectStorageModelFromStore(id, modelStage); err != nil {
		return objectStorageModelErrResp(c, status, err)
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
		return objectStorageModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to save the lkvstore to file : [%v]", err))
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusOK, "Succeeded in Deleting the object storage model")
}
