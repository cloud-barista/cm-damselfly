package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"github.com/google/uuid"
	echo "github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	"github.com/rs/zerolog/log"

	model 			"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-beetle/imdl/lineage"
	onpremisemodel 	"github.com/cloud-barista/cm-beetle/imdl/on-premise-model"
	cloudmodel 		"github.com/cloud-barista/cm-beetle/imdl/cloud-model"
)

type ModelsVersionRespInfo struct {
	OnPremModelVer        []string `json:"onpremModelVersion"`
	CloudModelVer         []string `json:"cloudModelVersion"`
	SoftwareModelVer      []string `json:"softwareModelVersion"`
	RDBMSModelVer         []string `json:"rdbmsModelVersion"`
	ObjectStorageModelVer []string `json:"objectStorageModelVersion"`
}

type GetModelsVersionResp struct {
	ModelsVersion ModelsVersionRespInfo `json:"modelsVersion"`
}

const (
	OnPremModel        string = "OnPremiseModel"
	CloudModel         string = "CloudModel"
	SWModel            string = "SoftwareModel"
	RDBMSModel         string = "RDBMSModel"
	ObjectStorageModel string = "ObjectStorageModel"
)

// formerObjectStorageModel is the 'modelType' of the object storage models stored before v0.6.6.
const formerObjectStorageModel string = "objectStorageModel"

// GetModelsVersions godoc
// @ID GetModelsVersions
// @Summary Get all tagged versions of the migration models
// @Description Get all tagged versions (in ascending order) of the migration models. 'onpremModelVersion' and 'cloudModelVersion' are the 'imdl/vX.Y.Z' tags of cm-beetle, 'softwareModelVersion' is the 'smdl/vX.Y.Z' tags of cm-grasshopper, 'rdbmsModelVersion' is the 'imdl/vX.Y.Z' tags of cm-beetle which have the RDBMS model (v0.1.13 or later), and 'objectStorageModelVersion' is the 'imdl/vX.Y.Z' tags of cm-beetle which have the object storage model (v0.1.3 or later).
// @Tags [API] Migration Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetModelsVersionResp "Tagged migration model versions"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /model/version [get]
func GetModelsVersions(c echo.Context) error {
	infraVersions, err := getInfraModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    "Model versions unavailable",
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	softwareVersions, err := getSoftwareModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-grasshopper/smdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    "Model versions unavailable",
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	modelsVersionInfo := ModelsVersionRespInfo{
		OnPremModelVer:        infraVersions,
		CloudModelVer:         infraVersions,
		SoftwareModelVer:      softwareVersions,
		RDBMSModelVer:         filterRDBMSModelVersions(infraVersions),
		ObjectStorageModelVer: filterObjectStorageModelVersions(infraVersions),
	}
	res := GetModelsVersionResp{
		ModelsVersion: modelsVersionInfo,
	}
	return c.JSON(http.StatusOK, res)
}

// ##############################################################################################
// ### On-premise, Cloud, Software Migration User Model
// ##############################################################################################

// GetModels godoc
// @ID GetModels
// @Summary Get a list of migration user models (infra, software, RDBMS and object storage)
// @Description Get a list of migration user models of all categories (infra, software, rdbms and objectStorage).
// @Description Use 'modelStage' to select the stage (source, planning or target) of the models, and 'category' to select the category. If omitted, the models of all stages (or all categories) are returned.
// @Description Multiple stages can be given by commas (e.g. 'modelStage=source,planning').
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceGroupId' to get all the models derived from a source group).
// @Description 'category' of each model (infra, software, rdbms or objectStorage) has the same values as the 'category' filter of 'GET /model'.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelStage      query []string false "Stages of the models to retrieve. Multiple stages are separated by commas (e.g. source,planning). If empty, the models of all stages are returned." Enums(source, planning, target) collectionFormat(csv)
// @Param category        query string false "Category of models to retrieve. If empty, the models of all categories are returned." Enums(infra, software, rdbms, objectStorage)
// @Param sourceGroupId   query string false "Filter by 'lineage.sourceGroupId'"
// @Param sourceModelId   query string false "Filter by 'lineage.sourceModelId'"
// @Param planningModelId query string false "Filter by 'lineage.planningModelId'"
// @Param targetModelId   query string false "Filter by 'lineage.targetModelId'"
// @Success 200 {array}  map[string]interface{} "Successfully obtained migration user models"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 500 {object} model.Response
// @Router /model [get]
func GetModels(c echo.Context) error {
	// 'modelStage' is optional. If it is empty, the models of all stages are returned.
	modelStages, err := parseModelStagesParam(c, false)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	// 'category' is optional. If it is empty, the models of all categories are returned.
	var category modelCategory
	if name := strings.TrimSpace(c.QueryParam("category")); name != "" {
		parsed, ok := parseModelCategory(name)
		if !ok {
			newErr := fmt.Errorf("invalid request: 'category' must be '%s', '%s', '%s' or '%s', got '%s'",
				categoryInfra, categorySoftware, categoryRDBMS, categoryObjectStorage, c.QueryParam("category"))
			log.Warn().Msg(newErr.Error())
			return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: fmt.Sprintf("'category' must be '%s', '%s', '%s' or '%s'",
				categoryInfra, categorySoftware, categoryRDBMS, categoryObjectStorage)})
		}
		category = parsed
	}

	lineageFilter := parseLineageFilter(c)
	log.Info().Msgf("# GetModels: modelStage=%v, category=[%s], lineage=[%+v]", modelStages, category, lineageFilter)

	modelList, _ := lkvstore.GetWithPrefix("")
	return c.JSON(http.StatusOK, filterModels(modelList, modelStages, category, lineageFilter))
}

// filterModels returns the stored models of the given stages and category whose lineage matches the filter,
// in the form of decorateModel. No stage or an empty category matches all stages or categories.
// The models without a valid category or stage are skipped.
func filterModels(modelList []interface{}, stages []ModelStage, category modelCategory, filter lineage.MigrationLineage) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, item := range modelList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		c, ok := storedModelCategory(m)
		if !ok || (category != "" && c != category) {
			continue
		}
		s, ok := storedModelStage(m)
		if !ok || (len(stages) > 0 && !slices.Contains(stages, s)) {
			continue
		}
		if !matchLineageFilter(storedLineage(m, s), filter) {
			continue
		}
		result = append(result, decorateModel(m))
	}
	return result
}

// ##############################################################################################
// ### Unified Infra Migration User Model (On-Premise + Cloud)
// ##############################################################################################

// GetInfraModels godoc
// @ID GetInfraModels
// @Summary Get a list of infra migration user models (on-premise or cloud)
// @Description Get a list of infra migration user models filtered by model type and model stage. Multiple stages can be given by commas (e.g. 'modelStage=source,planning').
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceModelId' to find the models derived from a source model).
// @Description 'category' of each model (infra, software, rdbms or objectStorage) has the same values as the 'category' filter of 'GET /model'.
// @Tags [API] Migration User Models - Infra
// @Accept  json
// @Produce  json
// @Param modelType  query string true  "Type of infra model to retrieve" Enums(onprem, cloud)
// @Param modelStage query []string true "Stages of the models to retrieve. Multiple stages are separated by commas (e.g. source,planning)." Enums(source, planning, target) collectionFormat(csv)
// @Param sourceGroupId   query string false "Filter by 'lineage.sourceGroupId'"
// @Param sourceModelId   query string false "Filter by 'lineage.sourceModelId'"
// @Param planningModelId query string false "Filter by 'lineage.planningModelId'"
// @Param targetModelId   query string false "Filter by 'lineage.targetModelId'"
// @Success 200 {array}  map[string]interface{} "Successfully obtained infra migration user models"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /infra-model [get]
func GetInfraModels(c echo.Context) error {
	modelTypeParam := c.QueryParam("modelType")
	if !strings.EqualFold(modelTypeParam, "onprem") && !strings.EqualFold(modelTypeParam, "cloud") {
		newErr := fmt.Errorf("invalid request: 'modelType' must be 'onprem' or 'cloud', got '%s'", modelTypeParam)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    "'modelType' must be 'onprem' or 'cloud'",
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	modelStages, err := parseModelStagesParam(c, true)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	isCloudModelFilter := strings.EqualFold(modelTypeParam, "cloud")
	lineageFilter := parseLineageFilter(c)

	log.Info().Msgf("# GetInfraModels: modelType=[%s], modelStage=%v, lineage=[%+v]", modelTypeParam, modelStages, lineageFilter)

	modelList, _ := lkvstore.GetWithPrefix("")
	result := []map[string]interface{}{}
	for _, m := range filterModels(modelList, modelStages, categoryInfra, lineageFilter) {
		if m["isCloudModel"] == isCloudModelFilter {
			result = append(result, m)
		}
	}
	return c.JSON(http.StatusOK, result)
}

// [Note]
// CreateInfraModelReq is a unified request body for creating either an on-premise
// or a cloud infra migration user model. Only the field matching the selected
// 'modelType' needs to be provided (onpremiseInfraModel or cloudInfraModel).
type CreateInfraModelReq struct {
	UserId           string                      `json:"userId"`
	IsInitUserModel  bool                        `json:"isInitUserModel"`
	UserModelName    string                      `json:"userModelName"`
	UserModelVer     string                      `json:"userModelVersion"`
	Description      string                      `json:"description"`
	CSP              string                      `json:"csp,omitempty"`
	Region           string                      `json:"region,omitempty"`
	Zone             string                      `json:"zone,omitempty"`
	ModelLineageReq
	OnpremInfraModel onpremisemodel.OnpremInfra  `json:"onpremiseInfraModel,omitempty"`
	CloudInfraModel  cloudmodel.RecommendedInfra `json:"cloudInfraModel,omitempty"`
}

// [Note]
// infraModelRawReq is the actual request body of CreateInfraModel/UpdateInfraModel. The infra model is kept as
// raw JSON (shadowing the typed fields of CreateInfraModelReq) so that it can be validated against the struct of
// the selected cm-beetle/imdl tagged version instead of the version compiled into cm-damselfly.
type infraModelRawReq struct {
	CreateInfraModelReq
	OnpremInfraModel json.RawMessage `json:"onpremiseInfraModel,omitempty"`
	CloudInfraModel  json.RawMessage `json:"cloudInfraModel,omitempty"`
}

// onPremModelMeta holds the metadata of a stored on-premise migration user model.
type onPremModelMeta struct {
	Id              string `json:"id"`
	UserId          string `json:"userId"`
	IsInitUserModel bool   `json:"isInitUserModel"`
	UserModelName   string `json:"userModelName"`
	UserModelVer    string `json:"userModelVersion"`
	Description     string `json:"description"`
	OnPremModelVer  string `json:"onpremModelVersion"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
	IsCloudModel    bool   `json:"isCloudModel"`
	ModelType       string `json:"modelType"`
}

// cloudModelMeta holds the metadata of a stored cloud migration user model.
type cloudModelMeta struct {
	Id              string `json:"id"`
	UserId          string `json:"userId"`
	IsInitUserModel bool   `json:"isInitUserModel"`
	UserModelName   string `json:"userModelName"`
	Description     string `json:"description"`
	UserModelVer    string `json:"userModelVersion"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
	CSP             string `json:"csp"`
	Region          string `json:"region"`
	Zone            string `json:"zone"`
	IsCloudModel    bool   `json:"isCloudModel"`
	CloudModelVer   string `json:"cloudModelVersion"`
	ModelType       string `json:"modelType"`
}

// onPremModelRecord and cloudModelRecord are the stored forms of infra models whose
// 'onpremiseInfraModel' / 'cloudInfraModel' follow the struct of their own imdl version.
type onPremModelRecord struct {
	onPremModelMeta
	ModelStage       ModelStage               `json:"modelStage"`
	Lineage          lineage.MigrationLineage `json:"lineage"`
	OnpremInfraModel json.RawMessage          `json:"onpremiseInfraModel"`
}

type cloudModelRecord struct {
	cloudModelMeta
	ModelStage      ModelStage               `json:"modelStage"`
	Lineage         lineage.MigrationLineage `json:"lineage"`
	CloudInfraModel json.RawMessage          `json:"cloudInfraModel"`
}

// infraModelRecordResp responds with the record of an infra model just stored.
func infraModelRecordResp(c echo.Context, status int, record interface{}) error {
	resp, err := decorateRecord(record)
	if err != nil {
		log.Error().Msg(err.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: err.Error()})
	}
	return c.JSON(status, resp)
}

// CreateInfraModel godoc
// @ID CreateInfraModel
// @Summary Create a new infra migration user model (on-premise or cloud)
// @Description Create a new infra migration user model. Use 'modelType' to select on-premise or cloud, and 'modelStage' to select the stage (source, planning or target) of the model.
// @Description The 'onpremiseInfraModel' (or 'cloudInfraModel') is validated against the 'OnpremInfra' (or 'RecommendedInfra') struct of the cm-beetle/imdl tagged version given by 'modelVersion', and is stored in that struct's form.
// @Description The former query parameters 'onpremModelVersion' and 'cloudModelVersion' are no longer accepted (400 Bad Request). Use 'modelVersion' instead.
// @Description (The body schema below shows the struct of the cm-beetle/imdl version built into cm-damselfly. To see the struct of another version, choose 'imdl/{version}/doc.json' in 'Select a definition' at the top of Swagger UI.)
// @Description [Lineage] 'parentId' is the ID of the infra model this model is derived from, which must be in the previous stage (source → planning → target): a source model for a planning model, and a planning model for a target model. The upstream fields of 'lineage' are derived from the parent.
// @Description 'lineage' can also be given explicitly (e.g. 'sourceGroupId'). The field of the model's own stage (e.g. 'planningModelId' of a planning model) is set to the ID of the created model by the server.
// @Tags [API] Migration User Models - Infra
// @Accept  json
// @Produce  json
// @Param modelType  query string true  "Type of infra model to create" Enums(onprem, cloud)
// @Param modelStage query string true  "Stage of infra model to create" Enums(source, planning, target)
// @Param modelVersion query string false "Model version (a cm-beetle 'imdl/vX.Y.Z' tag, e.g. v0.1.15). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Param Model body CreateInfraModelReq true "Infra model information"
// @Success 201 {object} object "Successfully created the infra migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /infra-model [post]
func CreateInfraModel(c echo.Context) error {
	modelTypeParam := c.QueryParam("modelType")
	if !strings.EqualFold(modelTypeParam, "onprem") && !strings.EqualFold(modelTypeParam, "cloud") {
		newErr := fmt.Errorf("invalid request: 'modelType' must be 'onprem' or 'cloud', got '%s'", modelTypeParam)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    "'modelType' must be 'onprem' or 'cloud'",
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	reqBody := new(infraModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		newErr := fmt.Errorf("invalid request : [%v]", err)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    "Malformed request body: check JSON syntax",
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	isCloudModel := strings.EqualFold(modelTypeParam, "cloud")
	log.Info().Msgf("# CreateInfraModel: modelType=[%s], modelStage=[%s]", modelTypeParam, modelStage)

	// The former parameters are rejected instead of being ignored, because an ignored version would silently select the latest tag.
	for _, formerParam := range []string{"onpremModelVersion", "cloudModelVersion"} {
		if c.QueryParams().Has(formerParam) {
			newErr := fmt.Errorf("invalid request: '%s' is no longer supported. Use 'modelVersion' instead", formerParam)
			log.Warn().Msg(newErr.Error())
			return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: fmt.Sprintf("'%s' no longer supported: use 'modelVersion'", formerParam)})
		}
	}
	requestedVer := strings.TrimSpace(c.QueryParam("modelVersion"))

	id := uuid.New().String()

	createTime, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
	}

	infraVersions, err := getInfraModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: "Model versions unavailable"})
	}

	resultVer, err := selectModelVersion(requestedVer, infraVersions)
	if err != nil {
		newErr := fmt.Errorf("invalid infra model version: %w", err)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	rawInfraModel := reqBody.OnpremInfraModel
	if isCloudModel {
		rawInfraModel = reqBody.CloudInfraModel
	}
	infraModel, status, err := normalizeInfraModel(c.Request().Context(), resultVer, isCloudModel, rawInfraModel)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(status, model.Response{Success: false, Text: err.Error()})
	}

	modelLineage, status, err := resolveLineage(lookupStoredModel, id, categoryInfra, modelStage, lineage.MigrationLineage{}, reqBody.ModelLineageReq)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(status, model.Response{Success: false, Text: err.Error()})
	}

	if isCloudModel {
		// --- Create a Cloud migration user model ---
		userModel := cloudModelRecord{cloudModelMeta: cloudModelMeta{
			Id:              id,
			UserId:          reqBody.UserId,
			IsInitUserModel: reqBody.IsInitUserModel,
			UserModelName:   reqBody.UserModelName,
			Description:     reqBody.Description,
			UserModelVer:    reqBody.UserModelVer,
			CreateTime:      createTime,
			CSP:             reqBody.CSP,
			Region:          reqBody.Region,
			Zone:            reqBody.Zone,
			IsCloudModel:    true,
			CloudModelVer:   resultVer,
			ModelType:       CloudModel,
		}, ModelStage: modelStage, Lineage: modelLineage, CloudInfraModel: infraModel}
		log.Info().Msgf("Cloud Model version: %s", resultVer)

		lkvstore.Put(userModel.Id, userModel)

		if err := lkvstore.SaveLkvStore(); err != nil {
			msg := "Failed to Save the lkvstore to file."
			log.Error().Msgf("%s : [%v]", msg, err)
			return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: "Model save failed"})
		}
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")

		return infraModelRecordResp(c, http.StatusCreated, userModel)
	}

	// --- Create an On-premise migration user model ---
	userModel := onPremModelRecord{onPremModelMeta: onPremModelMeta{
		Id:               id,
		UserId:           reqBody.UserId,
		IsInitUserModel:  reqBody.IsInitUserModel,
		UserModelName:    reqBody.UserModelName,
		UserModelVer:     reqBody.UserModelVer,
		Description:      reqBody.Description,
		OnPremModelVer:   resultVer,
		CreateTime:       createTime,
		IsCloudModel:     false,
		ModelType:        OnPremModel,
	}, ModelStage: modelStage, Lineage: modelLineage, OnpremInfraModel: infraModel}
	log.Info().Msgf("On-premise Model version: %s", resultVer)

	lkvstore.Put(userModel.Id, userModel)

	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: "Model save failed"})
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return infraModelRecordResp(c, http.StatusCreated, userModel)
}

// GetInfraModel godoc
// @ID GetInfraModel
// @Summary Get a specific infra migration user model (on-premise or cloud)
// @Description Get a specific infra migration user model by ID. Use 'modelType' to specify whether it is an on-premise or cloud model.
// @Description The 'lineage' of the model refers to its upstream models and itself.
// @Description 'category' of the model (infra, software, rdbms or objectStorage) has the same values as the 'category' filter of 'GET /model'.
// @Tags [API] Migration User Models - Infra
// @Accept  json
// @Produce  json
// @Param id          path  string true "Model ID"
// @Param modelType  query string true  "Type of infra model to retrieve" Enums(onprem, cloud)
// @Success 200 {object} object "Successfully obtained the infra migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /infra-model/{id} [get]
func GetInfraModel(c echo.Context) error {
	m, _, status, err := getStoredInfraModel(c, "GetInfraModel")
	if err != nil {
		return c.JSON(status, model.Response{Success: false, Text: err.Error()})
	}
	return c.JSON(http.StatusOK, m)
}

// getStoredInfraModel validates the 'id' and 'modelType' parameters and returns the stored infra model
// in the form of decorateModel, and whether it is a cloud model.
// On failure, it also returns the HTTP status code to respond with.
func getStoredInfraModel(c echo.Context, apiName string) (map[string]interface{}, bool, int, error) {
	if strings.EqualFold(c.Param("id"), "") {
		newErr := fmt.Errorf("Model ID required")
		log.Warn().Msg(newErr.Error())
		return nil, false, http.StatusBadRequest, newErr
	}

	modelTypeParam := c.QueryParam("modelType")
	if !strings.EqualFold(modelTypeParam, "onprem") && !strings.EqualFold(modelTypeParam, "cloud") {
		newErr := fmt.Errorf("'modelType' must be 'onprem' or 'cloud'")
		log.Warn().Msg(newErr.Error())
		return nil, false, http.StatusBadRequest, newErr
	}

	isCloudModelFilter := strings.EqualFold(modelTypeParam, "cloud")
	log.Info().Msgf("# %s: id=[%s], modelType=[%s]", apiName, c.Param("id"), modelTypeParam)

	userModel, exists := lkvstore.Get(c.Param("id"))
	if !exists {
		newErr := fmt.Errorf("Model [%s] not found", c.Param("id"))
		log.Error().Msg(newErr.Error())
		return nil, false, http.StatusNotFound, newErr
	}

	m, ok := userModel.(map[string]interface{})
	if !ok {
		newErr := fmt.Errorf("Model data unreadable")
		log.Error().Msg(newErr.Error())
		return nil, false, http.StatusInternalServerError, newErr
	}

	// --- Verify modelType matches the stored model ---
	isCloudModelVal, cloudModelExists := m["isCloudModel"]
	if !cloudModelExists {
		newErr := fmt.Errorf("Model is neither an on-premise nor a cloud infra model")
		log.Warn().Msg(newErr.Error())
		return nil, false, http.StatusBadRequest, newErr
	}
	isCloudModelBool, ok := isCloudModelVal.(bool)
	if !ok {
		newErr := fmt.Errorf("Model is neither an on-premise nor a cloud infra model")
		log.Warn().Msg(newErr.Error())
		return nil, false, http.StatusBadRequest, newErr
	}
	if isCloudModelBool != isCloudModelFilter {
		expected := "onprem"
		if isCloudModelBool {
			expected = "cloud"
		}
		newErr := fmt.Errorf("Model [%s] is a '%s' model, not '%s'", c.Param("id"), expected, modelTypeParam)
		log.Warn().Msg(newErr.Error())
		return nil, false, http.StatusBadRequest, newErr
	}

	return decorateModel(m), isCloudModelFilter, 0, nil
}

// ConvertInfraModel godoc
// @ID ConvertInfraModel
// @Summary Convert a stored infra migration user model to a newer model version
// @Description Get a stored infra migration user model converted to the struct of the cm-beetle/imdl tagged version given by 'modelVersion'. The stored model is not changed.
// @Description Only the conversion to the same or a newer version is allowed. A cloud model cannot be converted to a version older than v0.1.2, and a model whose stored version is not a cm-beetle/imdl tag cannot be converted.
// @Description The response has the fields of 'GET /infra-model/{id}', with the model version and 'onpremiseInfraModel' (or 'cloudInfraModel') converted, and 'conversion' added: 'originVersion' (the stored version), 'currentVersion' (the version of the returned model), 'converted' (false if the stored version is already the requested version), 'droppedFields' (the paths of the fields removed because the requested version does not have them) and 'warnings' (what could not be converted as it was).
// @Description The values are not changed by the conversion unless a field's structure requires it. e.g. the disk sizes are kept as they are although their unit was changed from GiB to GB in the comments of v0.1.11.
// @Description [Migrations] v0.1.22: 'targetInfra.postCommands' of a cloud model is converted to 'targetInfra.postScript.phases'. 'postCommandAsync' and 'installMonAgent' were removed, so they are dropped (with a warning if they were true / yes).
// @Tags [API] Migration User Models - Infra
// @Accept  json
// @Produce  json
// @Param id           path  string true  "Model ID"
// @Param modelType    query string true  "Type of infra model to convert" Enums(onprem, cloud)
// @Param modelVersion query string false "Model version to convert to (a cm-beetle 'imdl/vX.Y.Z' tag, e.g. v0.1.19). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Success 200 {object} object "Successfully obtained the converted infra migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter, or the model cannot be converted to the version"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /infra-model/{id}/convert [get]
func ConvertInfraModel(c echo.Context) error {
	m, isCloudModel, status, err := getStoredInfraModel(c, "ConvertInfraModel")
	if err != nil {
		return c.JSON(status, model.Response{Success: false, Text: err.Error()})
	}

	infraVersions, err := getInfraModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: "Model versions unavailable"})
	}

	currentVersion, err := selectModelVersion(c.QueryParam("modelVersion"), infraVersions)
	if err != nil {
		newErr := fmt.Errorf("invalid infra model version: %w", err)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	schema, versionField := onpremInfraModelSchema, "onpremModelVersion"
	if isCloudModel {
		schema, versionField = cloudInfraModelSchema, "cloudModelVersion"
	}

	originVersion, _ := m[versionField].(string)
	if err := validateInfraModelConversion(isCloudModel, originVersion, currentVersion, infraVersions); err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	converted := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		converted[k] = v
	}

	if originVersion == currentVersion {
		converted["conversion"] = InfraModelConversion{
			OriginVersion:  originVersion,
			CurrentVersion: currentVersion,
			Converted:      false,
			DroppedFields:  []string{},
			Warnings:       []string{},
		}
		return c.JSON(http.StatusOK, converted)
	}

	if m[schema.FieldName] == nil {
		newErr := fmt.Errorf("the stored model with id [%s] has no '%s' to convert", c.Param("id"), schema.FieldName)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: fmt.Sprintf("Model [%s] has no '%s' to convert", c.Param("id"), schema.FieldName)})
	}
	rawInfraModel, err := json.Marshal(m[schema.FieldName])
	if err != nil {
		newErr := fmt.Errorf("failed to read the stored '%s' : [%v]", schema.FieldName, err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: "Model data unreadable"})
	}

	pkg, err := modelschema.LoadPackage(c.Request().Context(), infraModelSchemaSource, currentVersion, schema.PkgDir)
	if err != nil {
		newErr := fmt.Errorf("failed to load the cm-beetle/imdl %s '%s' model : [%v]", currentVersion, schema.PkgDir, err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: fmt.Sprintf("Model schema for version %s unavailable", currentVersion)})
	}

	infraModel, conversion, err := convertInfraModel(pkg, isCloudModel, originVersion, currentVersion, rawInfraModel)
	if err != nil {
		newErr := fmt.Errorf("failed to convert the model with id [%s] from %s to %s : [%v]", c.Param("id"), originVersion, currentVersion, err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: fmt.Sprintf("Model [%s] conversion from %s to %s failed", c.Param("id"), originVersion, currentVersion)})
	}
	log.Info().Msgf("Converted the model [%s] from %s to %s (dropped fields: %d, warnings: %d)", c.Param("id"), originVersion, currentVersion, len(conversion.DroppedFields), len(conversion.Warnings))

	converted[versionField] = currentVersion
	converted[schema.FieldName] = infraModel
	converted["conversion"] = conversion
	return c.JSON(http.StatusOK, converted)
}

// UpdateInfraModel godoc
// @ID UpdateInfraModel
// @Summary Update a specific infra migration user model (on-premise or cloud)
// @Description Update a specific infra migration user model by ID. Use 'modelType' to specify whether it is an on-premise or cloud model.
// @Description The model stage is changed to 'modelStage' if it is given, and is preserved otherwise.
// @Description The model version is preserved, and 'onpremiseInfraModel' (or 'cloudInfraModel') is validated against the struct of that cm-beetle/imdl tagged version.
// @Description [Lineage] The stored 'lineage' is preserved if neither 'lineage' nor 'parentId' is given. If 'lineage' is given, it replaces the stored one. If 'parentId' is given, the upstream fields are derived from that model in the previous stage (source → planning → target).
// @Description The stage cannot be changed while the model is referred to by the lineage of other models (409 Conflict with the referring models in 'list').
// @Tags [API] Migration User Models - Infra
// @Accept  json
// @Produce  json
// @Param id          path  string true "Model ID"
// @Param modelType  query string true  "Type of infra model to update" Enums(onprem, cloud)
// @Param modelStage query string false "Stage to change the infra model to. If empty, the stored stage is preserved." Enums(source, planning, target)
// @Param Model body CreateInfraModelReq true "Infra model information to update"
// @Success 200 {object} object "Successfully updated the infra migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The stage cannot be changed: the model is referred to by the lineage of other models"
// @Failure 500 {object} model.Response
// @Router /infra-model/{id} [put]
func UpdateInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		newErr := fmt.Errorf("Model ID required")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	reqId := c.Param("id")

	modelTypeParam := c.QueryParam("modelType")
	if !strings.EqualFold(modelTypeParam, "onprem") && !strings.EqualFold(modelTypeParam, "cloud") {
		newErr := fmt.Errorf("'modelType' must be 'onprem' or 'cloud'")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// 'modelStage' is optional. If it is empty, the stored stage is preserved.
	var requestedStage ModelStage
	if strings.TrimSpace(c.QueryParam("modelStage")) != "" {
		stage, err := parseModelStageParam(c)
		if err != nil {
			log.Warn().Msg(err.Error())
			return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
		}
		requestedStage = stage
	}

	isCloudModelFilter := strings.EqualFold(modelTypeParam, "cloud")
	log.Info().Msgf("# UpdateInfraModel: id=[%s], modelType=[%s], modelStage=[%s]", reqId, modelTypeParam, requestedStage)

	reqBody := new(infraModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		newErr := fmt.Errorf("invalid request : [%v]", err)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    "Malformed request body: check JSON syntax",
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	existingRaw, exists := lkvstore.Get(reqId)
	if !exists {
		newErr := fmt.Errorf("Model [%s] not found", reqId)
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}

	existing, ok := existingRaw.(map[string]interface{})
	if !ok {
		newErr := fmt.Errorf("Model data unreadable")
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	// --- Verify modelType matches the stored model ---
	isCloudModelVal, cloudModelExists := existing["isCloudModel"]
	if !cloudModelExists {
		newErr := fmt.Errorf("Model is neither an on-premise nor a cloud infra model")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	isCloudModelBool, ok := isCloudModelVal.(bool)
	if !ok {
		newErr := fmt.Errorf("Model is neither an on-premise nor a cloud infra model")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	if isCloudModelBool != isCloudModelFilter {
		expected := "onprem"
		if isCloudModelBool {
			expected = "cloud"
		}
		newErr := fmt.Errorf("Model [%s] is a '%s' model, not '%s'", reqId, expected, modelTypeParam)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	storedStage, hasStoredStage := storedModelStage(existing)
	modelStage := storedStage
	if requestedStage != "" {
		modelStage = requestedStage
		if requestedStage != storedStage {
			log.Info().Msgf("Changing the model stage of [%s]: [%s] -> [%s]", reqId, storedStage, requestedStage)
		}
	} else if !hasStoredStage {
		newErr := fmt.Errorf("Model [%s] has no valid 'modelStage': specify 'modelStage' to set it", reqId)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: newErr.Error()})
	}

	// The stage of a model referred to by other models is not changed, so as not to break their lineage.
	if hasStoredStage && modelStage != storedStage {
		if referrers := findLineageReferrers(reqId); len(referrers) > 0 {
			return lineageConflictResp(c, reqId, "change the stage of", referrers)
		}
	}

	modelLineage, status, err := resolveLineage(lookupStoredModel, reqId, categoryInfra, modelStage, storedLineage(existing, storedStage), reqBody.ModelLineageReq)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(status, model.Response{Success: false, Text: err.Error()})
	}

	createTime, _ := existing["createTime"].(string)

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	preservedVer, _ := existing["onpremModelVersion"].(string)
	rawInfraModel := reqBody.OnpremInfraModel
	if isCloudModelFilter {
		preservedVer, _ = existing["cloudModelVersion"].(string)
		rawInfraModel = reqBody.CloudInfraModel
	}

	infraVersions, err := getInfraModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: "Model versions unavailable"})
	}

	var infraModel json.RawMessage
	if _, verErr := selectModelVersion(preservedVer, infraVersions); preservedVer != "" && verErr == nil {
		infraModel, status, err = normalizeInfraModel(c.Request().Context(), preservedVer, isCloudModelFilter, rawInfraModel)
	} else {
		log.Warn().Msgf("The stored model version [%s] is not a cm-beetle/imdl tag. Using the built-in model struct.", preservedVer)
		infraModel, status, err = normalizeInfraModelWithCompiledStruct(isCloudModelFilter, rawInfraModel)
	}
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(status, model.Response{Success: false, Text: err.Error()})
	}

	if isCloudModelFilter {
		cloudModelVer := preservedVer

		updatedModel := cloudModelRecord{cloudModelMeta: cloudModelMeta{
			Id:              reqId,
			UserId:          reqBody.UserId,
			IsInitUserModel: reqBody.IsInitUserModel,
			UserModelName:   reqBody.UserModelName,
			Description:     reqBody.Description,
			UserModelVer:    reqBody.UserModelVer,
			CreateTime:      createTime,
			UpdateTime:      updateTime,
			CSP:             reqBody.CSP,
			Region:          reqBody.Region,
			Zone:            reqBody.Zone,
			IsCloudModel:    true,
			CloudModelVer:   cloudModelVer,
			ModelType:       CloudModel,
		}, ModelStage: modelStage, Lineage: modelLineage, CloudInfraModel: infraModel}
		log.Info().Msgf("Cloud Model version (preserved): %s", cloudModelVer)

		lkvstore.Put(reqId, updatedModel)
		if err := lkvstore.SaveLkvStore(); err != nil {
			msg := "Failed to Save the lkvstore to file."
			log.Error().Msgf("%s : [%v]", msg, err)
			res := model.Response{
				Success: false,
				Text:    "Model save failed",
			}
			return c.JSON(http.StatusInternalServerError, res)
		}
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")

		return infraModelRecordResp(c, http.StatusOK, updatedModel)
	}

	onpremModelVer := preservedVer

	updatedModel := onPremModelRecord{onPremModelMeta: onPremModelMeta{
		Id:               reqId,
		UserId:           reqBody.UserId,
		IsInitUserModel:  reqBody.IsInitUserModel,
		UserModelName:    reqBody.UserModelName,
		UserModelVer:     reqBody.UserModelVer,
		Description:      reqBody.Description,
		OnPremModelVer:   onpremModelVer,
		CreateTime:       createTime,
		UpdateTime:       updateTime,
		IsCloudModel:     false,
		ModelType:        OnPremModel,
	}, ModelStage: modelStage, Lineage: modelLineage, OnpremInfraModel: infraModel}
	log.Info().Msgf("On-premise Model version (preserved): %s", onpremModelVer)

	lkvstore.Put(reqId, updatedModel)
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		res := model.Response{
			Success: false,
			Text:    "Model save failed",
		}
		return c.JSON(http.StatusInternalServerError, res)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return infraModelRecordResp(c, http.StatusOK, updatedModel)
}

// DeleteInfraModel godoc
// @ID DeleteInfraModel
// @Summary Delete a specific infra migration user model (on-premise or cloud)
// @Description Delete a specific infra migration user model by ID. Works for both on-premise and cloud models without requiring the caller to specify the model type.
// @Description If the model is referred to by the lineage of other models, it is not deleted (409 Conflict with the referring models in 'list') unless 'force' is true.
// @Tags [API] Migration User Models - Infra
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param force query boolean false "Delete the model even if it is referred to by the lineage of other models" default(false)
// @Success 200 {string} string "Successfully deleted the infra migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 409 {object} model.Response "The model is referred to by the lineage of other models (listed in 'list')"
// @Failure 500 {object} model.Response
// @Router /infra-model/{id} [delete]
func DeleteInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		newErr := fmt.Errorf("Model ID required")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	force, err := parseForceParam(c)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	log.Info().Msgf("# DeleteInfraModel: id=[%s], force=[%t]", c.Param("id"), force)

	existingRaw, exists := lkvstore.Get(c.Param("id"))
	if !exists {
		newErr := fmt.Errorf("Model [%s] not found", c.Param("id"))
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}

	existing, ok := existingRaw.(map[string]interface{})
	if !ok {
		newErr := fmt.Errorf("Model data unreadable")
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	// --- Verify this is an infra model (not a software model) ---
	if isSoftwareModel, exists := existing["isSoftwareModel"]; exists && isSoftwareModel == true {
		newErr := fmt.Errorf("Model [%s] is a software model, not an infra model", c.Param("id"))
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	if _, cloudModelExists := existing["isCloudModel"]; !cloudModelExists {
		newErr := fmt.Errorf("Model [%s] is neither an on-premise nor a cloud infra model", c.Param("id"))
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// --- Verify that no other model refers to this model in its lineage ---
	if referrers := findLineageReferrers(c.Param("id")); len(referrers) > 0 {
		if !force {
			return lineageConflictResp(c, c.Param("id"), "delete", referrers)
		}
		log.Warn().Msgf("Deleting the model [%s] referred to by the lineage of %d model(s) (force=true)", c.Param("id"), len(referrers))
	}

	lkvstore.Delete(c.Param("id"))
	log.Info().Msgf("Succeeded in Deleting the model : [%s]", c.Param("id"))

	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		res := model.Response{
			Success: false,
			Text:    "Model save failed",
		}
		return c.JSON(http.StatusInternalServerError, res)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusOK, "Infra model deleted")
}
