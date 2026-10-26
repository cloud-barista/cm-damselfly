package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"errors"
	"strings"
	"github.com/google/uuid"
	echo "github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/rs/zerolog/log"

	model 			"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-beetle/imdl/lineage"
	onpremisemodel 	"github.com/cloud-barista/cm-beetle/imdl/on-premise-model"
	cloudmodel 		"github.com/cloud-barista/cm-beetle/imdl/cloud-model"
)

type ModelsVersionRespInfo struct {
	OnPremModelVer   []string `json:"onpremModelVersion"`
	CloudModelVer    []string `json:"cloudModelVersion"`
	SoftwareModelVer []string `json:"softwareModelVersion"`
}

type GetModelsVersionResp struct {
	ModelsVersion ModelsVersionRespInfo `json:"modelsVersion"`
}

const (
	OnPremModel string = "OnPremiseModel"
	CloudModel  string = "CloudModel"
	SWModel     string = "SoftwareModel"
)

// GetModelsVersions godoc
// @ID GetModelsVersions
// @Summary Get all tagged versions of the migration models
// @Description Get all tagged versions (in ascending order) of the migration models. 'onpremModelVersion' and 'cloudModelVersion' are the 'imdl/vX.Y.Z' tags of cm-beetle, and 'softwareModelVersion' is the 'smdl/vX.Y.Z' tags of cm-grasshopper.
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
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	softwareVersions, err := getSoftwareModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-grasshopper/smdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	modelsVersionInfo := ModelsVersionRespInfo{
		OnPremModelVer:   infraVersions,
		CloudModelVer:    infraVersions,
		SoftwareModelVer: softwareVersions,
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
// @Summary Get a list of migration user models (infra and software)
// @Description Get a list of migration user models of both categories (infra and software).
// @Description Use 'modelStage' to select the stage (source, planning or target) of the models, and 'category' to select the category. If omitted, the models of all stages (or both categories) are returned.
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceGroupId' to get all the models derived from a source group).
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelStage      query string false "Stage of models to retrieve. If empty, the models of all stages are returned." Enums(source, planning, target)
// @Param category        query string false "Category of models to retrieve. If empty, the models of both categories are returned." Enums(infra, software)
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
	var modelStage ModelStage
	if strings.TrimSpace(c.QueryParam("modelStage")) != "" {
		stage, err := parseModelStageParam(c)
		if err != nil {
			log.Warn().Msg(err.Error())
			return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
		}
		modelStage = stage
	}

	// 'category' is optional. If it is empty, the models of both categories are returned.
	category := modelCategory(strings.ToLower(strings.TrimSpace(c.QueryParam("category"))))
	if category != "" && category != categoryInfra && category != categorySoftware {
		newErr := fmt.Errorf("invalid request: 'category' must be '%s' or '%s', got '%s'", categoryInfra, categorySoftware, c.QueryParam("category"))
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: newErr.Error()})
	}

	lineageFilter := parseLineageFilter(c)
	log.Info().Msgf("# GetModels: modelStage=[%s], category=[%s], lineage=[%+v]", modelStage, category, lineageFilter)

	modelList, _ := lkvstore.GetWithPrefix("")
	return c.JSON(http.StatusOK, filterModels(modelList, modelStage, category, lineageFilter))
}

// filterModels returns the stored models of the given stage and category whose lineage matches the filter,
// with 'modelStage' and 'lineage' filled in. An empty stage or category matches all stages or categories.
// The models without a valid category or stage are skipped.
func filterModels(modelList []interface{}, stage ModelStage, category modelCategory, filter lineage.MigrationLineage) []map[string]interface{} {
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
		if !ok || (stage != "" && s != stage) {
			continue
		}
		if !matchLineageFilter(storedLineage(m, s), filter) {
			continue
		}
		result = append(result, withLineage(withModelStage(m, s), s))
	}
	return result
}

// ##############################################################################################
// ### On-premise Migration User Model
// ##############################################################################################

type OnPremModelReqInfo struct {
	UserId           string             		`json:"userId"`
	IsInitUserModel  bool               		`json:"isInitUserModel"`
	UserModelName    string             		`json:"userModelName"`
	UserModelVer     string             		`json:"userModelVersion"`
	Description      string             		`json:"description"`
	OnpremInfraModel onpremisemodel.OnpremInfra `json:"onpremiseInfraModel" validate:"required"`
}

type OnPremModelRespInfo struct {
	Id               string             		`json:"id"`
	UserId           string             		`json:"userId"`
	IsInitUserModel  bool               		`json:"isInitUserModel"`
	UserModelName    string             		`json:"userModelName"`
	UserModelVer     string             		`json:"userModelVersion"`
	Description      string             		`json:"description"`
	OnPremModelVer   string             		`json:"onpremModelVersion"`
	CreateTime       string             		`json:"createTime"`
	UpdateTime       string             		`json:"updateTime"`
	IsTargetModel    bool               		`json:"isTargetModel"`
	IsCloudModel     bool               		`json:"isCloudModel"`
	ModelType 		 string                  	`json:"modelType"`
	OnpremInfraModel onpremisemodel.OnpremInfra `json:"onpremiseInfraModel" validate:"required"`
}

// ##############################################################################################
// ### Unified Infra Migration User Model (On-Premise + Cloud)
// ##############################################################################################

// GetInfraModels godoc
// @ID GetInfraModels
// @Summary Get a list of infra migration user models (on-premise or cloud)
// @Description Get a list of infra migration user models filtered by model type and model stage.
// @Description The models can be further filtered by their 'lineage' (e.g. 'sourceModelId' to find the models derived from a source model).
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelType  query string true  "Type of infra model to retrieve" Enums(onprem, cloud)
// @Param modelStage query string true  "Stage of infra model to retrieve" Enums(source, planning, target)
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
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	modelStage, err := parseModelStageParam(c)
	if err != nil {
		log.Warn().Msg(err.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: err.Error()})
	}

	isCloudModelFilter := strings.EqualFold(modelTypeParam, "cloud")
	lineageFilter := parseLineageFilter(c)

	log.Info().Msgf("# GetInfraModels: modelType=[%s], modelStage=[%s], lineage=[%+v]", modelTypeParam, modelStage, lineageFilter)

	modelList, exists := lkvstore.GetWithPrefix("")
	if !exists {
		return c.JSON(http.StatusOK, []map[string]interface{}{})
	}

	var result []map[string]interface{}
	for _, item := range modelList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Skip software models
		if isSoftwareModel, exists := m["isSoftwareModel"]; exists && isSoftwareModel == true {
			continue
		}

		// Filter by isCloudModel
		isCloudModelVal, cloudModelExists := m["isCloudModel"]
		if !cloudModelExists {
			continue
		}
		isCloudModelBool, ok := isCloudModelVal.(bool)
		if !ok {
			continue
		}
		if isCloudModelBool != isCloudModelFilter {
			continue
		}

		// Filter by modelStage
		stage, ok := storedModelStage(m)
		if !ok || stage != modelStage {
			continue
		}

		// Filter by lineage
		if !matchLineageFilter(storedLineage(m, stage), lineageFilter) {
			continue
		}

		result = append(result, withLineage(withModelStage(m, stage), stage))
	}

	if len(result) == 0 {
		return c.JSON(http.StatusOK, []map[string]interface{}{})
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

// onPremModelRecord and cloudModelRecord are the stored forms of infra models whose
// 'onpremiseInfraModel' / 'cloudInfraModel' follow the struct of their own imdl version.
// 'IsTargetModel' shadows the one of the embedded struct so that it is omitted for the planning stage.
type onPremModelRecord struct {
	OnPremModelRespInfo
	ModelStage       ModelStage               `json:"modelStage"`
	IsTargetModel    *bool                    `json:"isTargetModel,omitempty"`
	Lineage          lineage.MigrationLineage `json:"lineage"`
	OnpremInfraModel json.RawMessage          `json:"onpremiseInfraModel"`
}

type cloudModelRecord struct {
	CloudModelRespInfo
	ModelStage      ModelStage               `json:"modelStage"`
	IsTargetModel   *bool                    `json:"isTargetModel,omitempty"`
	Lineage         lineage.MigrationLineage `json:"lineage"`
	CloudInfraModel json.RawMessage          `json:"cloudInfraModel"`
}

// CreateInfraModel godoc
// @ID CreateInfraModel
// @Summary Create a new infra migration user model (on-premise or cloud)
// @Description Create a new infra migration user model. Use 'modelType' to select on-premise or cloud, and 'modelStage' to select the stage (source, planning or target) of the model.
// @Description The 'onpremiseInfraModel' (or 'cloudInfraModel') is validated against the 'OnpremInfra' (or 'RecommendedInfra') struct of the cm-beetle/imdl tagged version given by 'onpremModelVersion' (or 'cloudModelVersion'), and is stored in that struct's form.
// @Description (The body schema below shows the struct of the cm-beetle/imdl version built into cm-damselfly. To see the struct of another version, choose 'imdl/{version}/doc.json' in 'Select a definition' at the top of Swagger UI.)
// @Description [Lineage] 'parentId' is the ID of the infra model this model is derived from, which must be in the previous stage (source → planning → target): a source model for a planning model, and a planning model for a target model. The upstream fields of 'lineage' are derived from the parent.
// @Description 'lineage' can also be given explicitly (e.g. 'sourceGroupId'). The field of the model's own stage (e.g. 'planningModelId' of a planning model) is set to the ID of the created model by the server.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param modelType  query string true  "Type of infra model to create" Enums(onprem, cloud)
// @Param modelStage query string true  "Stage of infra model to create" Enums(source, planning, target)
// @Param onpremModelVersion query string false "On-premise model version (a cm-beetle 'imdl/vX.Y.Z' tag, e.g. v0.1.15). Only for modelType=onprem (ignored for modelType=cloud only if it equals 'cloudModelVersion'). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Param cloudModelVersion query string false "Cloud model version (a cm-beetle 'imdl/vX.Y.Z' tag, e.g. v0.1.15). Only for modelType=cloud (ignored for modelType=onprem only if it equals 'onpremModelVersion'). If empty, the latest tag is used. See 'GET /model/version' for available versions."
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
			Text:    newErr.Error(),
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
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	isCloudModel := strings.EqualFold(modelTypeParam, "cloud")
	log.Info().Msgf("# CreateInfraModel: modelType=[%s], modelStage=[%s]", modelTypeParam, modelStage)

	onpremModelVerParam := strings.TrimSpace(c.QueryParam("onpremModelVersion"))
	cloudModelVerParam := strings.TrimSpace(c.QueryParam("cloudModelVersion"))
	requestedVer := onpremModelVerParam
	// The parameter of the other model type is accepted only if it is the same version
	// (both are pre-filled with the same version in the per-version API definition of Swagger UI).
	if isCloudModel {
		requestedVer = cloudModelVerParam
		if onpremModelVerParam != "" && onpremModelVerParam != cloudModelVerParam {
			newErr := fmt.Errorf("invalid request: 'onpremModelVersion' cannot be used with modelType 'cloud'. Use 'cloudModelVersion' instead")
			log.Warn().Msg(newErr.Error())
			return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: newErr.Error()})
		}
	} else if cloudModelVerParam != "" && cloudModelVerParam != onpremModelVerParam {
		newErr := fmt.Errorf("invalid request: 'cloudModelVersion' cannot be used with modelType 'onprem'. Use 'onpremModelVersion' instead")
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: newErr.Error()})
	}

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
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}

	resultVer, err := selectModelVersion(requestedVer, infraVersions)
	if err != nil {
		newErr := fmt.Errorf("invalid infra model version: %w", err)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusBadRequest, model.Response{Success: false, Text: newErr.Error()})
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
		userModel := cloudModelRecord{CloudModelRespInfo: CloudModelRespInfo{
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
		}, ModelStage: modelStage, IsTargetModel: modelStage.legacyIsTargetModel(), Lineage: modelLineage, CloudInfraModel: infraModel}
		log.Info().Msgf("Cloud Model version: %s", resultVer)

		lkvstore.Put(userModel.Id, userModel)

		if err := lkvstore.SaveLkvStore(); err != nil {
			msg := "Failed to Save the lkvstore to file."
			log.Error().Msgf("%s : [%v]", msg, err)
			newErr := fmt.Errorf("%s : [%v]", msg, err)
			return c.JSON(http.StatusInternalServerError, newErr)
		}
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")

		return c.JSON(http.StatusCreated, userModel)
	}

	// --- Create an On-premise migration user model ---
	userModel := onPremModelRecord{OnPremModelRespInfo: OnPremModelRespInfo{
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
	}, ModelStage: modelStage, IsTargetModel: modelStage.legacyIsTargetModel(), Lineage: modelLineage, OnpremInfraModel: infraModel}
	log.Info().Msgf("On-premise Model version: %s", resultVer)

	lkvstore.Put(userModel.Id, userModel)

	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusInternalServerError, newErr)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusCreated, userModel)
}

// GetInfraModel godoc
// @ID GetInfraModel
// @Summary Get a specific infra migration user model (on-premise or cloud)
// @Description Get a specific infra migration user model by ID. Use 'modelType' to specify whether it is an on-premise or cloud model.
// @Description The 'lineage' of the model refers to its upstream models and itself.
// @Tags [API] Migration User Models
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
	if strings.EqualFold(c.Param("id"), "") {
		newErr := fmt.Errorf("invalid request: model id is required")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	modelTypeParam := c.QueryParam("modelType")
	if !strings.EqualFold(modelTypeParam, "onprem") && !strings.EqualFold(modelTypeParam, "cloud") {
		newErr := fmt.Errorf("invalid request: 'modelType' must be 'onprem' or 'cloud', got '%s'", modelTypeParam)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	isCloudModelFilter := strings.EqualFold(modelTypeParam, "cloud")
	log.Info().Msgf("# GetInfraModel: id=[%s], modelType=[%s]", c.Param("id"), modelTypeParam)

	userModel, exists := lkvstore.Get(c.Param("id"))
	if !exists {
		newErr := fmt.Errorf("failed to find the model from db with id: [%s]", c.Param("id"))
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}

	m, ok := userModel.(map[string]interface{})
	if !ok {
		newErr := fmt.Errorf("internal error: unexpected model data format")
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	// --- Verify modelType matches the stored model ---
	isCloudModelVal, cloudModelExists := m["isCloudModel"]
	if !cloudModelExists {
		newErr := fmt.Errorf("'isCloudModel' does not exist. This is neither on-premise nor cloud user model")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	isCloudModelBool, ok := isCloudModelVal.(bool)
	if !ok {
		newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
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
		newErr := fmt.Errorf("model type mismatch: the model with id [%s] is a '%s' model, not '%s'", c.Param("id"), expected, modelTypeParam)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	if stage, ok := storedModelStage(m); ok {
		m = withLineage(withModelStage(m, stage), stage)
	}
	return c.JSON(http.StatusOK, m)
}

// UpdateInfraModel godoc
// @ID UpdateInfraModel
// @Summary Update a specific infra migration user model (on-premise or cloud)
// @Description Update a specific infra migration user model by ID. Use 'modelType' to specify whether it is an on-premise or cloud model.
// @Description The model stage is changed to 'modelStage' if it is given, and is preserved otherwise.
// @Description The model version is preserved, and 'onpremiseInfraModel' (or 'cloudInfraModel') is validated against the struct of that cm-beetle/imdl tagged version.
// @Description [Lineage] The stored 'lineage' is preserved if neither 'lineage' nor 'parentId' is given. If 'lineage' is given, it replaces the stored one. If 'parentId' is given, the upstream fields are derived from that model in the previous stage (source → planning → target).
// @Description The stage cannot be changed while the model is referred to by the lineage of other models (409 Conflict with the referring models in 'list').
// @Tags [API] Migration User Models
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
		newErr := fmt.Errorf("invalid request: model id is required")
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
		newErr := fmt.Errorf("invalid request: 'modelType' must be 'onprem' or 'cloud', got '%s'", modelTypeParam)
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
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	existingRaw, exists := lkvstore.Get(reqId)
	if !exists {
		newErr := fmt.Errorf("failed to find the model from db with id: [%s]", reqId)
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}

	existing, ok := existingRaw.(map[string]interface{})
	if !ok {
		newErr := fmt.Errorf("internal error: unexpected model data format")
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
		newErr := fmt.Errorf("'isCloudModel' does not exist. This is neither on-premise nor cloud user model")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	isCloudModelBool, ok := isCloudModelVal.(bool)
	if !ok {
		newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
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
		newErr := fmt.Errorf("model type mismatch: the model with id [%s] is a '%s' model, not '%s'", reqId, expected, modelTypeParam)
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
		newErr := fmt.Errorf("the model with id [%s] has neither a valid 'modelStage' nor 'isTargetModel'. Specify 'modelStage' to set it", reqId)
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
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
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

		updatedModel := cloudModelRecord{CloudModelRespInfo: CloudModelRespInfo{
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
		}, ModelStage: modelStage, IsTargetModel: modelStage.legacyIsTargetModel(), Lineage: modelLineage, CloudInfraModel: infraModel}
		log.Info().Msgf("Cloud Model version (preserved): %s", cloudModelVer)

		lkvstore.Put(reqId, updatedModel)
		if err := lkvstore.SaveLkvStore(); err != nil {
			msg := "Failed to Save the lkvstore to file."
			log.Error().Msgf("%s : [%v]", msg, err)
			newErr := fmt.Errorf("%s : [%v]", msg, err)
			res := model.Response{
				Success: false,
				Text:    newErr.Error(),
			}
			return c.JSON(http.StatusInternalServerError, res)
		}
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")

		saved, _ := lkvstore.Get(reqId)
		return c.JSON(http.StatusOK, saved)
	}

	onpremModelVer := preservedVer

	updatedModel := onPremModelRecord{OnPremModelRespInfo: OnPremModelRespInfo{
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
	}, ModelStage: modelStage, IsTargetModel: modelStage.legacyIsTargetModel(), Lineage: modelLineage, OnpremInfraModel: infraModel}
	log.Info().Msgf("On-premise Model version (preserved): %s", onpremModelVer)

	lkvstore.Put(reqId, updatedModel)
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	saved, _ := lkvstore.Get(reqId)
	return c.JSON(http.StatusOK, saved)
}

// DeleteInfraModel godoc
// @ID DeleteInfraModel
// @Summary Delete a specific infra migration user model (on-premise or cloud)
// @Description Delete a specific infra migration user model by ID. Works for both on-premise and cloud models without requiring the caller to specify the model type.
// @Description If the model is referred to by the lineage of other models, it is not deleted (409 Conflict with the referring models in 'list') unless 'force' is true.
// @Tags [API] Migration User Models
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
		newErr := fmt.Errorf("invalid request: model id is required")
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
		newErr := fmt.Errorf("failed to find the model from db with id: [%s]", c.Param("id"))
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}

	existing, ok := existingRaw.(map[string]interface{})
	if !ok {
		newErr := fmt.Errorf("internal error: unexpected model data format")
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	// --- Verify this is an infra model (not a software model) ---
	if isSoftwareModel, exists := existing["isSoftwareModel"]; exists && isSoftwareModel == true {
		newErr := fmt.Errorf("invalid request: the model with id [%s] is a software model, not an infra model", c.Param("id"))
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	if _, cloudModelExists := existing["isCloudModel"]; !cloudModelExists {
		newErr := fmt.Errorf("invalid request: the model with id [%s] is neither an on-premise nor a cloud infra model", c.Param("id"))
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
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusOK, "Succeeded in Deleting the infra model")
}

type GetOnPremModelsResp struct {
	Models []OnPremModelRespInfo `json:"models"`
}

// GetOnPremModels godoc
// @ID GetOnPremModels
// @Summary Get a list of on-premise models
// @Description Get a list of on-premise models.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetOnPremModelsResp "Successfully Obtained On-Premise Migration User Models"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /onpremmodel [get]
func GetOnPremModels(c echo.Context) error {
	var onpremModels []map[string]interface{}
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		// # Returns Only On-prem Models
		for _, model := range modelList {
			if model, ok := model.(map[string]interface{}); ok {
				if isCloudModel, exists := model["isCloudModel"]; exists && isCloudModel == false {

					// if id, exists := model["id"]; exists {
					// 	// fmt.Printf("Loaded value-1 for [%s]: %v", c.Param("id"), model)
					// 	if id, ok := id.(string); ok {
					// 		log.Debug().Msgf("# Model ID to Add : [%s]", id)
					// 	} else {
					// 		msg := ("'id' is not a string type")
					// 		log.Error().Msg(msg)

					// 		newErr := fmt.Errorf("'id' is not a string type")
					// 		return c.JSON(http.StatusNotFound, newErr)
					// 	}
					// } else {
					// 	msg := ("'id' does not exist")
					// 	log.Error().Msg(msg)

					// 	newErr := fmt.Errorf("'id' does not exist")
					// 	return c.JSON(http.StatusNotFound, newErr)
					// }

					onpremModels = append(onpremModels, model)
				}
			}
		}

		return c.JSON(http.StatusOK, onpremModels)
	} else {
		return c.JSON(http.StatusOK, nil)
	}
}

type GetOnPremModelResp struct {
	OnPremModelRespInfo
}

// GetOnPremModel godoc
// @ID GetOnPremModel
// @Summary Get a specific on-premise model
// @Description Get a specific on-premise model.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetOnPremModelResp "Successfully Obtained the On-Premise Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /onpremmodel/{id} [get]
func GetOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		newErr := fmt.Errorf("Invalid request, invalid model id")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	log.Info().Msgf("# Model ID to Get : [%s]", c.Param("id"))

	userModel, exists := lkvstore.Get(c.Param("id"))
	if exists {
		// log.Info().Msgf("Loaded value for [%s]: %v", c.Param("id"), value)

		if userModel, ok := userModel.(map[string]interface{}); ok {
			// Check if the userModel is a on-premise userModel
			if isCloudModel, exists := userModel["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						newErr := fmt.Errorf("invalid request, invalid on-premise userModel id")
						log.Warn().Msg(newErr.Error())
						res := model.Response{
							Success: false,
							Text:    newErr.Error(),
						}
						return c.JSON(http.StatusBadRequest, res)
					} else {
						log.Info().Msg("This userModel is a On-premise Model!!")
					}
				} else {		
					newErr := fmt.Errorf("isCloudModel' is not a boolean type")
					log.Warn().Msg(newErr.Error())
					res := model.Response{
						Success: false,
						Text:    newErr.Error(),
					}
					return c.JSON(http.StatusBadRequest, res)
				}
			} else {
				newErr := fmt.Errorf("'isCloudModel' does not exist. This is neither on-premise user model nor cloud user model.")
				log.Warn().Msg(newErr.Error())
				res := model.Response{
					Success: false,
					Text:    newErr.Error(),
				}
				return c.JSON(http.StatusBadRequest, res)
			}
		}

		return c.JSON(http.StatusOK, userModel)
	} else {
		newErr := fmt.Errorf("Failed to find the model from db with the id")
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of OnPremModel
type CreateOnPremModelReq struct {
	OnPremModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of OnPremModel
type CreateOnPremModelResp struct {
	OnPremModelRespInfo
}

// CreateOnPremModel godoc
// @ID CreateOnPremModel
// @Summary Create a new on-premise model
// @Description Create a new on-premise model with the given information.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateOnPremModelReq true "model information"
// @Success 201 {object} CreateOnPremModelResp "Successfully Created the On-Premise Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /onpremmodel [post]
func CreateOnPremModel(c echo.Context) error {
	userModel := new(CreateOnPremModelResp)

	if err := c.Bind(userModel); err != nil {
		newErr := fmt.Errorf("invalid request : [%v]", err)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	// fmt.Println("### OnPremModel",)
	// spew.Dump(userModel)

	userModel.Id = uuid.New().String()

	time, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	}
	userModel.CreateTime 	= time
	userModel.IsTargetModel = false
	userModel.IsCloudModel 	= false
	userModel.ModelType 	= OnPremModel

	resultVer, err := getLatestInfraModelVersion()
	if err != nil {
		newErr := fmt.Errorf("failed to get the latest tagged cm-beetle/imdl model version: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}
	userModel.OnPremModelVer = resultVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the userModel to the key-value store
	lkvstore.Put(userModel.Id, userModel)

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusCreated, userModel)
}

// [Note]
// Struct Embedding is used to inherit the fields of OnPremModel
type UpdateOnPremModelReq struct {
	OnPremModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of OnPremModel
type UpdateOnPremModelResp struct {
	OnPremModelRespInfo
}

// UpdateOnPremModel godoc
// @ID UpdateOnPremModel
// @Summary Update a on-premise model
// @Description Update a on-premise model with the given information.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateOnPremModelReq true "Model information to update"
// @Success 201 {object} UpdateOnPremModelResp "Successfully Updated the On-Premise Migration User Model"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /onpremmodel/{id} [put]
func UpdateOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		msg := "Invalid ID!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	reqId := c.Param("id")
	log.Info().Msgf("# Model ID to Update : [%s]", reqId)

	updateModel := new(UpdateOnPremModelResp)

	model, exists := lkvstore.Get(reqId)
	if exists {
		log.Info().Msgf("Succeeded in Finding the model : [%s]", reqId)

		if err := c.Bind(updateModel); err != nil {
			msg := "Invalid Request!!"
			log.Error().Msg(msg)
			newErr := errors.New(msg)
			return c.JSON(http.StatusBadRequest, newErr)
		}

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						msg := "The Given ID is Not a On-premise Model ID"
						log.Error().Msgf("%s : [%s]", msg, reqId)
						newErr := fmt.Errorf("%s : [%s]", msg, reqId)
						return c.JSON(http.StatusNotFound, newErr)
					} else {
						log.Info().Msg("This model is a On-premise Model!!")
					}
				} else {
					msg := "'isCloudModel' is not a boolean type"
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isCloudModel' does not exist. This is neither on-premise user model nor cloud user model."
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if onPremModelVer, exists := model["onpremModelVersion"]; exists {
				if onpremModelVerStr, ok := onPremModelVer.(string); ok {
					updateModel.OnPremModelVer = onpremModelVerStr
					log.Info().Msgf("On-premise Model version : [%s]", onpremModelVerStr)
				} else {
					msg := "'onpremModelVersion' is not a string type of value"
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'onpremModelVersion' does not exist"
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}

		}
		// else {
		// 	msg := "Error!! Error!! Error!! Error!! Error!!"
		// 	log.Error().Msg(msg)
		// }

		if model, ok := model.(map[string]interface{}); ok {
			if createTime, exists := model["createTime"]; exists {
				if createTimeStr, ok := createTime.(string); ok {
					// fmt.Printf("### createTimeStr : [%s]", createTimeStr)
					updateModel.CreateTime = createTimeStr
				} else {
					msg := "'createTime' is not a string type of value"
					log.Debug().Msg(msg)
					// newErr := errors.New(msg)
					// return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'createTime' does not exist"
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		// onpremModelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
		// if err != nil {
		// 	fmt.Println("Error:", err)
		// } else {
		// 	fmt.Printf("On-premise Model version: %s", onpremModelVer)
		// }
		// updateModel.OnPremModelVer = onpremModelVer

		updateModel.Id = reqId

		time, err := getSeoulCurrentTime()
		if err != nil {
			msg := "Failed to Get the Current time!!"
			log.Debug().Msg(msg)
			// newErr := errors.New(msg)
			// return c.JSON(http.StatusNotFound, newErr)
		}
		updateModel.UpdateTime = time
		updateModel.IsTargetModel = false
		updateModel.IsCloudModel = false

		// Convert to String type
		// strNum := strconv.Itoa(id)

		// Save the model to the key-value store
		lkvstore.Put(reqId, updateModel)

		// # Save the current state of the key-value store to file (Memory to file)
		if err := lkvstore.SaveLkvStore(); err != nil {
			msg := "Failed to Save the lkvstore to file."
			log.Error().Msgf("%s : [%v]", msg, err)
			newErr := fmt.Errorf("%s : [%v]", msg, err)
			return c.JSON(http.StatusNotFound, newErr)
		} else {
			log.Info().Msg("Succeeded in Saving the lkvstore to file.")
		}

		// return c.JSON(http.StatusCreated, updateModel)
		// => Not http.StatusCreated

		// Get the model from the DB
		model, exists := lkvstore.Get(reqId)
		if exists {
			// log.Info().Msgf("Loaded value for [%s]: %v", c.Param("id"), model)
			return c.JSON(http.StatusOK, model)
		} else {
			msg := "Failed to Find the Model from DB with the ID"
			log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
			newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
			return c.JSON(http.StatusNotFound, newErr)
		}
	} else {
		msg := "Failed to Find the Model from DB"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// No RequestBody required for "DELETE /onpremmodel/{id}"

// [Note]
// No ResponseBody required for "DELETE /onpremmodel/{id}"

// DeleteOnPremModel godoc
// @ID DeleteOnPremModel
// @Summary Delete a on-premise model
// @Description Delete a on-premise model with the given information.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {string} string "Successfully Deleted the On-Premise Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /onpremmodel/{id} [delete]
func DeleteOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		msg := "Invalid ID!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	log.Info().Msgf("# Model ID to Delete : [%s]", c.Param("id"))

	// Verify loaded data without prefix
	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		log.Info().Msgf("Succeeded in Finding the model : [%s]", c.Param("id"))

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						msg := "The Given ID is Not a On-premise Model ID"
						log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
						newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					} else {
						log.Info().Msg("This model is a Cloud Model!!")
					}
				} else {
					msg := "'isCloudModel' is not a boolean type"
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isCloudModel' does not exist. This is neither on-premise user model nor cloud user model."
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		lkvstore.Delete(c.Param("id"))		
	} else {
		msg := "Failed to Find the Model from DB "
		log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
		newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusOK, "Succeeded in Deleting the model")
}

// ##############################################################################################
// ### Cloud Migration User Model
// ##############################################################################################

type CloudModelReqInfo struct {
	UserId          string                  		`json:"userId"`
	IsTargetModel   bool                    		`json:"isTargetModel"`
	IsInitUserModel bool                    		`json:"isInitUserModel"`
	UserModelName   string                  		`json:"userModelName"`
	Description     string                  		`json:"description"`
	UserModelVer    string                  		`json:"userModelVersion"`
	CSP             string                  		`json:"csp"`
	Region          string                  		`json:"region"`
	Zone            string                  		`json:"zone"`
	CloudInfraModel cloudmodel.RecommendedInfra 	`json:"cloudInfraModel" validate:"required"`
}

type CloudModelRespInfo struct {
	Id              string                  		`json:"id"`
	UserId          string                  		`json:"userId"`
	IsTargetModel   bool                    		`json:"isTargetModel"`
	IsInitUserModel bool                   			`json:"isInitUserModel"`
	UserModelName   string                  		`json:"userModelName"`
	Description     string                  		`json:"description"`
	UserModelVer    string                  		`json:"userModelVersion"`
	CreateTime      string                  		`json:"createTime"`
	UpdateTime      string                  		`json:"updateTime"`
	CSP             string                  		`json:"csp"`
	Region          string                  		`json:"region"`
	Zone            string                  		`json:"zone"`
	IsCloudModel    bool                    		`json:"isCloudModel"`
	CloudModelVer   string                  		`json:"cloudModelVersion"`
	ModelType 		 string                  	   	`json:"modelType"`
	CloudInfraModel cloudmodel.RecommendedInfra 	`json:"cloudInfraModel" validate:"required"`
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetCloudModelsResp struct {
	Models []CloudModelRespInfo `json:"models"`
}

// GetCloudModels godoc
// @ID GetCloudModels
// @Summary Get a list of cloud user models
// @Description Get a list of cloud user models.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetCloudModelsResp "Successfully Obtained Cloud Migration User Models"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /cloudmodel [get]
func GetCloudModels(c echo.Context) error {
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		//  Returns Only Cloud Models
		var cloudModels []map[string]interface{}
		for _, model := range modelList {
			// fmt.Printf("# Model value : %v", model)
			if model, ok := model.(map[string]interface{}); ok {
				if isCloudModel, exists := model["isCloudModel"]; exists && isCloudModel == true {
					cloudModels = append(cloudModels, model)
				}
			}
		}

		if len(cloudModels) < 1 {
			return c.JSON(http.StatusOK, nil)
		}

		return c.JSON(http.StatusOK, cloudModels)
	} else {
		return c.JSON(http.StatusOK, nil)
	}
}

type GetCloudModelResp struct {
	CloudModelRespInfo
}

// GetCloudModel godoc
// @ID GetCloudModel
// @Summary Get a specific cloud user model
// @Description Get a specific cloud user model.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetCloudModelResp "Successfully Obtained the Cloud Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /cloudmodel/{id} [get]
func GetCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		msg := "Invalid ID!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	log.Info().Msgf("# Model ID to Get : [%s]", c.Param("id"))

	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		// log.Info().Msgf("# Loaded value for [%s]: %v", c.Param("id"), model)

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						log.Info().Msg("This model is a Cloud Model!!")
					} else {
						msg := "The Given ID is Not a Cloud Model ID"
						log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
						newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
					msg := ("'isCloudModel' is not a boolean type")
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isCloudModel' does not exist. This is neither on-premise user model nor cloud user model."
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		return c.JSON(http.StatusOK, model)
	} else {
		msg := "Failed to Find the Model from DB with the ID"
		log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
		newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of CloudModel
type CreateCloudModelReq struct {
	CloudModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of CloudModel
type CreateCloudModelResp struct {
	CloudModelRespInfo
}

// CreateCloudModel godoc
// @ID CreateCloudModel
// @Summary Create a new cloud user model
// @Description Create a new cloud user model with the given information.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateCloudModelReq true "model information"
// @Success 201 {object} CreateCloudModelResp "Successfully Created the Cloud Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /cloudmodel [post]
func CreateCloudModel(c echo.Context) error {
	userModel := new(CreateCloudModelResp)

	if err := c.Bind(userModel); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	// fmt.Println("### CreateCloudModelResp",)
	// spew.Dump(userModel)

	userModel.Id = uuid.New().String()

	time, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	}
	userModel.CreateTime 	= time
	userModel.IsCloudModel 	= true
	userModel.ModelType 	= CloudModel

	resultVer, err := getLatestInfraModelVersion()
	if err != nil {
		newErr := fmt.Errorf("failed to get the latest tagged cm-beetle/imdl model version: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}
	userModel.CloudModelVer = resultVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the userModel to the key-value store
	lkvstore.Put(userModel.Id, userModel)

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusInternalServerError, newErr)
	} else {
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusCreated, userModel)
}

// [Note]
// Struct Embedding is used to inherit the fields of CloudModel
type UpdateCloudModelReq struct {
	CloudModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of CloudModel
type UpdateCloudModelResp struct {
	CloudModelRespInfo
}

// UpdateCloudModel godoc
// @ID UpdateCloudModel
// @Summary Update a cloud user model
// @Description Update a cloud user model with the given information.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateCloudModelReq true "Model information to update"
// @Success 201 {object} UpdateCloudModelResp "Successfully Updated the Cloud Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /cloudmodel/{id} [put]
func UpdateCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		err := fmt.Errorf("invalid id")
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	reqId := c.Param("id")
	log.Info().Msgf("# Model ID to Update : [%s]", reqId)

	updateModel := new(UpdateCloudModelResp)

	if err := c.Bind(updateModel); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	// fmt.Printf("New Req Values for [%s]: %v", c.Param("id"), updateModel)

	model, exists := lkvstore.Get(reqId)
	if exists {
		log.Info().Msgf("Succeeded in Finding the model : [%s]", reqId)
		// fmt.Printf("Values from DB [%s]: %v", c.Param("id"), model)

		if cloudModel, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := cloudModel["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					log.Info().Msgf("The value of isCloudModel is: %v", isCloudModel)

					if isCloudModelBool {
						log.Info().Msg("This model is a Cloud Model!!")
					} else {
						msg := "The Given ID is Not a Cloud Model ID"
						log.Error().Msgf("%s : [%s]", msg, reqId)
						newErr := fmt.Errorf("%s : [%s]", msg, reqId)
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
		
					msg := "'isCloudModel' is not a boolean type"
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)			

				}
			} else {
				msg := "'isCloudModel' does not exist. This is neither on-premise user model nor cloud user model."
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if cloudModelVer, exists := model["cloudModelVersion"]; exists {
				if cloudModelVerStr, ok := cloudModelVer.(string); ok {

					updateModel.CloudModelVer = cloudModelVerStr

					log.Info().Msgf("# cloudModelVer : [%s]", cloudModelVerStr)
				} else {
					log.Info().Msg("'cloudModelVersion' is not a string type of value")
				}
			} else {
				msg := "'cloudModelVersion' does not exist"
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if createTime, exists := model["createTime"]; exists {
				if createTimeStr, ok := createTime.(string); ok {
					updateModel.CreateTime = createTimeStr
				} else {
					msg := "'createTime' is not a string type of value"
					log.Debug().Msg(msg)
					// newErr := errors.New(msg)
					// return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'createTime' does not exist"
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		// cloudModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
		// if err != nil {
		// 	fmt.Println("Error:", err)
		// } else {
		// 	fmt.Printf("Cloud Model version: %s", cloudModelVer)
		// }
		// updateModel.CloudModelVer = cloudModelVer

		updateModel.Id = reqId
		time, err := getSeoulCurrentTime()
		if err != nil {
			msg := "Failed to Get the Current time!!"
			log.Debug().Msg(msg)
			// newErr := errors.New(msg)
			// return c.JSON(http.StatusNotFound, newErr)
		}
		updateModel.UpdateTime = time
		updateModel.IsCloudModel = true

		// fmt.Println("### updateModel",)
		// spew.Dump(updateModel)

		// Convert to String type
		// strNum := strconv.Itoa(id)

		// Save the model to the key-value store
		lkvstore.Put(reqId, updateModel)

		// # Save the current state of the key-value store to file
		if err := lkvstore.SaveLkvStore(); err != nil {
			msg := "Failed to Save the lkvstore to file."
			log.Error().Msgf("%s : [%v]", msg, err)
			newErr := fmt.Errorf("%s : [%v]", msg, err)
			return c.JSON(http.StatusNotFound, newErr)
		} else {
			log.Info().Msg("Succeeded in Saving the lkvstore to file.")
		}

		// Get the model from the DB
		model, exists := lkvstore.Get(reqId)
		if exists {
			// log.Info().Msgf("Loaded value for [%s]: %v", c.Param("id"), model)
			return c.JSON(http.StatusOK, model)
		} else {
			msg := "Failed to Find the Model from DB with the ID"
			log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
			newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
			return c.JSON(http.StatusNotFound, newErr)
		}
	} else {
		msg := "Failed to Find the Model from DB"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// No RequestBody required for "DELETE /cloudmodel/{id}"

// [Note]
// No ResponseBody required for "DELETE /cloudmodel/{id}"

// DeleteCloudModel godoc
// @ID DeleteCloudModel
// @Summary Delete a cloud user model
// @Description Delete a cloud user model with the given information.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {string} string "Successfully Deleted the Cloud Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /cloudmodel/{id} [delete]
func DeleteCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		msg := "Invalid ID!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	log.Info().Msgf("# Model ID to Delete : [%s]", c.Param("id"))

	// Verify loaded data without prefix
	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		log.Info().Msgf("Succeeded in Finding the model : [%s]", c.Param("id"))

		if model, ok := model.(map[string]interface{}); ok {
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok && isCloudModelBool {
					log.Info().Msg("This model is a Cloud Model!!")
				} else {
					msg := "The Given ID is Not a Cloud Model ID"
					log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
					newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isCloudModel' does not exist. This is neither on-premise user model nor cloud user model."
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		lkvstore.Delete(c.Param("id"))
	} else {
		msg := "Failed to Find the Model from DB with the ID"
		log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
		newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusOK, "Succeeded in Deleting the model")
}
