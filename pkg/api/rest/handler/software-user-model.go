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
	"github.com/rs/zerolog/log"
	// "github.com/davecgh/go-spew/spew"

	model 			"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	softwaremodel 	"github.com/cloud-barista/cm-grasshopper/smdl"
)

// ##############################################################################################
// ### Source Software Migration User Model
// ##############################################################################################

type SourceSoftwareModelReqInfo struct {
	UserId          	string                  		`json:"userId"`
	IsInitUserModel 	bool                    		`json:"isInitUserModel"`
	UserModelName   	string                  		`json:"userModelName"`
	UserModelVer    	string                  		`json:"userModelVersion"`
	Description     	string                  		`json:"description"`
	SourceSoftwareModel softwaremodel.SourceGroupSoftwareProperty	`json:"sourceSoftwareModel" validate:"required"`	
}

type SourceSoftwareModelRespInfo struct {
	Id              	string                  		`json:"id"`
	UserId          	string                  		`json:"userId"`
	NodeId          	string                  		`json:"nodeId,omitempty"`
	IsInitUserModel 	bool                   			`json:"isInitUserModel"`
	UserModelName   	string                  		`json:"userModelName"`
	UserModelVer    	string                  		`json:"userModelVersion"`
	Description     	string                  		`json:"description"`
	SoftwareModelVer 	string                  		`json:"softwareModelVersion"`	
	CreateTime      	string                  		`json:"createTime"`
	UpdateTime      	string                  		`json:"updateTime"`
	IsSoftwareModel     bool                    		`json:"isSoftwareModel"`
	IsTargetModel   	bool                    		`json:"isTargetModel"`
	ModelType 		 	string                  	   	`json:"modelType"`
	SourceSoftwareModel softwaremodel.SourceGroupSoftwareProperty	`json:"sourceSoftwareModel" validate:"required"`	
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetSourceSoftwareModelsResp struct {
	Models []SourceSoftwareModelRespInfo `json:"models"`
}

// GetSourceSoftwareModels godoc
// @ID GetSourceSoftwareModels
// @Summary Get a list of source software user models
// @Description Get a list of source software user models.
// @Tags [API] Source Software Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetSourceSoftwareModelsResp "Successfully Obtained Source Software Migration User Models"
// @Failure 404 {object} model.Response
// @Router /softwaremodel/source [get]
func GetSourceSoftwareModels(c echo.Context) error {
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		//  Returns Only Software Models
		var softwareModels []map[string]interface{}
		for _, model := range modelList {
			// fmt.Printf("# Model value : %v", model)
			if model, ok := model.(map[string]interface{}); ok {
				if isSoftwareModel, exists := model["isSoftwareModel"]; exists && isSoftwareModel == true {
					if isTargetModel, exists := model["isTargetModel"]; exists && isTargetModel == false {
						softwareModels = append(softwareModels, model)
					}
				}
			}
		}

		if len(softwareModels) < 1 {
			return c.JSON(http.StatusOK, nil)
		}

		return c.JSON(http.StatusOK, softwareModels)
	} else {
		return c.JSON(http.StatusOK, nil)
	}
}

type GetSourceSoftwareModelResp struct {
	SourceSoftwareModelRespInfo
}

// GetSourceSoftwareModel godoc
// @ID GetSourceSoftwareModel
// @Summary Get a specific source software user model
// @Description Get a specific source software user model.
// @Tags [API] Source Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetSourceSoftwareModelResp "Successfully Obtained the Source Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /softwaremodel/source/{id} [get]
func GetSourceSoftwareModel(c echo.Context) error {
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
			if isSoftwareModel, exists := model["isSoftwareModel"]; exists {
				if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok {
					if isSoftwareModelBool {
						log.Info().Msg("This model is a Software Model!!")
					} else {
						msg := "The Given ID is Not a Software Model ID"
						log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
						newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
					msg := ("'isSoftwareModel' is not a boolean type")
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isSoftwareModel' does not exist"
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
// Struct Embedding is used to inherit the fields of SoftwareModel
type CreateSourceSoftwareModelReq struct {
	SourceSoftwareModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of SoftwareModel
type CreateSourceSoftwareModelResp struct {
	SourceSoftwareModelRespInfo
}

// CreateSourceSoftwareModel godoc
// @ID CreateSourceSoftwareModel
// @Summary Create a new source software user model
// @Description Create a new source software user model with the given information.
// @Tags [API] Source Software Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateSourceSoftwareModelReq true "model information"
// @Success 201 {object} CreateSourceSoftwareModelResp "Successfully Created the Source Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Router /softwaremodel/source [post]
func CreateSourceSoftwareModel(c echo.Context) error {
	model := new(CreateSourceSoftwareModelResp)

	if err := c.Bind(model); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	// fmt.Println("### CreateSourceSoftwareModelResp",)
	// spew.Dump(model)

	randomStr := uuid.New().String()
	log.Info().Msgf("Generated UUID : [%s]", randomStr)
	model.Id = randomStr

	time, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	}
	model.CreateTime 		= time
	model.IsSoftwareModel 	= true
	model.IsTargetModel 	= false
	model.ModelType 		= SWModel

	resultVer, err := getLatestSoftwareModelVersion()
	if err != nil {
		newErr := fmt.Errorf("failed to get the latest tagged cm-grasshopper/smdl model version: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"success": false, "text": newErr.Error()})
	}
	model.SoftwareModelVer = resultVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the model to the key-value store
	lkvstore.Put(randomStr, model)

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusCreated, model)
}

// [Note]
// Struct Embedding is used to inherit the fields of SourceSoftwareModel
type UpdateSourceSoftwareModelReq struct {
	SourceSoftwareModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of SourceSoftwareModel
type UpdateSourceSoftwareModelResp struct {
	SourceSoftwareModelRespInfo
}

// UpdateSourceSoftwareModel godoc
// @ID UpdateSourceSoftwareModel
// @Summary Update a source software user model
// @Description Update a source software user model with the given information.
// @Tags [API] Source Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateSourceSoftwareModelReq true "Model information to update"
// @Success 201 {object} UpdateSourceSoftwareModelResp "Successfully Updated the Source Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /softwaremodel/source/{id} [put]
func UpdateSourceSoftwareModel(c echo.Context) error {
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

	// Bind the request to get the updated fields
	reqModel := new(UpdateSourceSoftwareModelReq)
	if err := c.Bind(reqModel); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}

	// Get the existing model from the store
	existingData, exists := lkvstore.Get(reqId)
	if !exists {
		msg := "Model not found"
		log.Error().Msgf("%s : [%s]", msg, reqId)
		newErr := fmt.Errorf("%s : [%s]", msg, reqId)
		return c.JSON(http.StatusNotFound, newErr)
	}

	// Verify it's a software model
	if softwareModel, ok := existingData.(map[string]interface{}); ok {
		if isSoftwareModel, exists := softwareModel["isSoftwareModel"]; exists {
			if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok {
				if !isSoftwareModelBool {
					msg := "The Given ID is Not a Software Model ID"
					log.Error().Msgf("%s : [%s]", msg, reqId)
					newErr := fmt.Errorf("%s : [%s]", msg, reqId)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isSoftwareModel' is not a boolean type"
				log.Debug().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusInternalServerError, newErr)
			}
		} else {
			msg := "'isSoftwareModel' does not exist"
			log.Error().Msg(msg)
			newErr := errors.New(msg)
			return c.JSON(http.StatusInternalServerError, newErr)
		}
	}

	// Unmarshal existing data into the full response model struct
	fullModel := new(CreateSourceSoftwareModelResp)
	jsonBytes, err := json.Marshal(existingData)
	if err != nil {
		msg := "Failed to marshal existing data"
		log.Error().Err(err).Msg(msg)
		return c.JSON(http.StatusInternalServerError, errors.New(msg))
	}
	if err := json.Unmarshal(jsonBytes, fullModel); err != nil {
		msg := "Failed to unmarshal existing data into model"
		log.Error().Err(err).Msg(msg)
		return c.JSON(http.StatusInternalServerError, errors.New(msg))
	}

	// Update only the fields provided in the request
	fullModel.UserId = reqModel.UserId
	fullModel.IsInitUserModel = reqModel.IsInitUserModel
	fullModel.UserModelName = reqModel.UserModelName
	fullModel.UserModelVer = reqModel.UserModelVer
	fullModel.Description = reqModel.Description
	fullModel.SourceSoftwareModel = reqModel.SourceSoftwareModel

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}
	fullModel.UpdateTime = updateTime

	// Save the updated full model back to the store
	lkvstore.Put(reqId, fullModel)

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusInternalServerError, newErr)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	log.Info().Msgf("Successfully updated the model: [%s]", reqId)
	return c.JSON(http.StatusOK, fullModel)
}

// [Note]
// No RequestBody required for "DELETE /softwaremodel/source/{id}"

// [Note]
// No ResponseBody required for "DELETE /softwaremodel/source/{id}"

// DeleteSourceSoftwareModel godoc
// @ID DeleteSourceSoftwareModel
// @Summary Delete a source software user model
// @Description Delete a source software user model with the given information.
// @Tags [API] Source Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {string} string "Successfully Deleted the Source Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /softwaremodel/source/{id} [delete]
func DeleteSourceSoftwareModel(c echo.Context) error {
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
			if isSoftwareModel, exists := model["isSoftwareModel"]; exists {
				if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok && isSoftwareModelBool {
					log.Info().Msg("This model is a Software Model!!")
				} else {
					msg := "The Given ID is Not a Software Model ID"
					log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
					newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isSoftwareModel' does not exist"
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

// ##############################################################################################
// ### Target Software Migration User Model
// ##############################################################################################

type TargetSoftwareModelReqInfo struct {
	UserId          	string                  		`json:"userId"`
	IsInitUserModel 	bool                    		`json:"isInitUserModel"`
	UserModelName   	string                  		`json:"userModelName"`
	UserModelVer    	string                  		`json:"userModelVersion"`
	Description     	string                  		`json:"description"`
	TargetSoftwareModel softwaremodel.TargetGroupSoftwareProperty	`json:"targetSoftwareModel" validate:"required"`	
}

type TargetSoftwareModelRespInfo struct {
	Id              	string                  		`json:"id"`
	UserId          	string                  		`json:"userId"`
	NodeId          	string                  		`json:"nodeId,omitempty"`
	IsInitUserModel 	bool                   			`json:"isInitUserModel"`
	UserModelName   	string                  		`json:"userModelName"`
	UserModelVer    	string                  		`json:"userModelVersion"`
	Description     	string                  		`json:"description"`
	SoftwareModelVer 	string                  		`json:"softwareModelVersion"`
	CreateTime      	string                  		`json:"createTime"`
	UpdateTime      	string                  		`json:"updateTime"`
	IsSoftwareModel     bool                    		`json:"isSoftwareModel"`
	IsTargetModel   	bool                    		`json:"isTargetModel"`
	ModelType 		 	string                  	   	`json:"modelType"`
	TargetSoftwareModel softwaremodel.TargetGroupSoftwareProperty	`json:"targetSoftwareModel" validate:"required"`	
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetTargetSoftwareModelsResp struct {
	Models []TargetSoftwareModelRespInfo `json:"models"`
}

// GetTargetSoftwareModels godoc
// @ID GetTargetSoftwareModels
// @Summary Get a list of target software user models
// @Description Get a list of target software user models.
// @Tags [API] Target Software Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetTargetSoftwareModelsResp "Successfully Obtained Target Software Migration User Models"
// @Failure 404 {object} model.Response
// @Router /softwaremodel/target [get]
func GetTargetSoftwareModels(c echo.Context) error {
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		//  Returns Only Software Models
		var softwareModels []map[string]interface{}
		for _, model := range modelList {
			// fmt.Printf("# Model value : %v", model)
			if model, ok := model.(map[string]interface{}); ok {
				if isSoftwareModel, exists := model["isSoftwareModel"]; exists && isSoftwareModel == true {
					if isTargetModel, exists := model["isTargetModel"]; exists && isTargetModel == true {
						softwareModels = append(softwareModels, model)
					}
				}
			}
		}

		if len(softwareModels) < 1 {
			return c.JSON(http.StatusOK, nil)
		}

		return c.JSON(http.StatusOK, softwareModels)
	} else {
		return c.JSON(http.StatusOK, nil)
	}
}

type GetTargetSoftwareModelResp struct {
	TargetSoftwareModelRespInfo
}

// GetTargetSoftwareModel godoc
// @ID GetTargetSoftwareModel
// @Summary Get a specific target software user model
// @Description Get a specific target software user model.
// @Tags [API] Target Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetTargetSoftwareModelResp "Successfully Obtained the Target Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /softwaremodel/target/{id} [get]
func GetTargetSoftwareModel(c echo.Context) error {
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
			if isSoftwareModel, exists := model["isSoftwareModel"]; exists {
				if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok {
					if isSoftwareModelBool {
						log.Info().Msg("This model is a Software Model!!")
					} else {
						msg := "The Given ID is Not a Software Model ID"
						log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
						newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
					msg := ("'isSoftwareModel' is not a boolean type")
					log.Debug().Msg(msg)
					newErr := errors.New(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isSoftwareModel' does not exist"
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
// Struct Embedding is used to inherit the fields of SoftwareModel
type CreateTargetSoftwareModelReq struct {
	TargetSoftwareModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of SoftwareModel
type CreateTargetSoftwareModelResp struct {
	TargetSoftwareModelRespInfo
}

// CreateTargetSoftwareModel godoc
// @ID CreateTargetSoftwareModel
// @Summary Create a new target software user model
// @Description Create a new target software user model with the given information.
// @Tags [API] Target Software Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateTargetSoftwareModelReq true "model information"
// @Success 201 {object} CreateTargetSoftwareModelResp "Successfully Created the Target Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Router /softwaremodel/target [post]
func CreateTargetSoftwareModel(c echo.Context) error {
	model := new(CreateTargetSoftwareModelResp)

	if err := c.Bind(model); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	// fmt.Println("### CreateTargetSoftwareModelResp",)
	// spew.Dump(model)

	randomStr := uuid.New().String()
	log.Info().Msgf("Generated UUID : [%s]", randomStr)
	model.Id = randomStr

	time, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	}
	model.CreateTime 		= time
	model.IsSoftwareModel 	= true
	model.IsTargetModel 	= true
	model.ModelType 		= SWModel

	resultVer, err := getLatestSoftwareModelVersion()
	if err != nil {
		newErr := fmt.Errorf("failed to get the latest tagged cm-grasshopper/smdl model version: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"success": false, "text": newErr.Error()})
	}
	model.SoftwareModelVer = resultVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the model to the key-value store
	lkvstore.Put(randomStr, model)

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msg("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusCreated, model)
}

// [Note]
// Struct Embedding is used to inherit the fields of TargetSoftwareModel
type UpdateTargetSoftwareModelReq struct {
	TargetSoftwareModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of TargetSoftwareModel
type UpdateTargetSoftwareModelResp struct {
	TargetSoftwareModelRespInfo
}

// UpdateTargetSoftwareModel godoc
// @ID UpdateTargetSoftwareModel
// @Summary Update a target software user model
// @Description Update a target software user model with the given information.
// @Tags [API] Target Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateTargetSoftwareModelReq true "Model information to update"
// @Success 201 {object} UpdateTargetSoftwareModelResp "Successfully Updated the Target Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /softwaremodel/target/{id} [put]
func UpdateTargetSoftwareModel(c echo.Context) error {
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

	// Bind the request to get the updated fields
	reqModel := new(UpdateTargetSoftwareModelReq)
	if err := c.Bind(reqModel); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}

	// Get the existing model from the store
	existingData, exists := lkvstore.Get(reqId)
	if !exists {
		msg := "Model not found"
		log.Error().Msgf("%s : [%s]", msg, reqId)
		newErr := fmt.Errorf("%s : [%s]", msg, reqId)
		return c.JSON(http.StatusNotFound, newErr)
	}

	// Verify it's a software model
	if softwareModel, ok := existingData.(map[string]interface{}); ok {
		if isSoftwareModel, exists := softwareModel["isSoftwareModel"]; exists {
			if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok {
				if !isSoftwareModelBool {
					msg := "The Given ID is Not a Software Model ID"
					log.Error().Msgf("%s : [%s]", msg, reqId)
					newErr := fmt.Errorf("%s : [%s]", msg, reqId)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isSoftwareModel' is not a boolean type"
				log.Debug().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusInternalServerError, newErr)
			}
		} else {
			msg := "'isSoftwareModel' does not exist"
			log.Error().Msg(msg)
			newErr := errors.New(msg)
			return c.JSON(http.StatusInternalServerError, newErr)
		}
	}

	// Unmarshal existing data into the full response model struct
	fullModel := new(CreateTargetSoftwareModelResp)
	jsonBytes, err := json.Marshal(existingData)
	if err != nil {
		msg := "Failed to marshal existing data"
		log.Error().Err(err).Msg(msg)
		return c.JSON(http.StatusInternalServerError, errors.New(msg))
	}
	if err := json.Unmarshal(jsonBytes, fullModel); err != nil {
		msg := "Failed to unmarshal existing data into model"
		log.Error().Err(err).Msg(msg)
		return c.JSON(http.StatusInternalServerError, errors.New(msg))
	}

	// Update only the fields provided in the request
	fullModel.UserId = reqModel.UserId
	fullModel.IsInitUserModel = reqModel.IsInitUserModel
	fullModel.UserModelName = reqModel.UserModelName
	fullModel.UserModelVer = reqModel.UserModelVer
	fullModel.Description = reqModel.Description
	fullModel.TargetSoftwareModel = reqModel.TargetSoftwareModel

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}
	fullModel.UpdateTime = updateTime

	// Save the updated full model back to the store
	lkvstore.Put(reqId, fullModel)

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		msg := "Failed to Save the lkvstore to file."
		log.Error().Msgf("%s : [%v]", msg, err)
		newErr := fmt.Errorf("%s : [%v]", msg, err)
		return c.JSON(http.StatusInternalServerError, newErr)
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	log.Info().Msgf("Successfully updated the model: [%s]", reqId)
	return c.JSON(http.StatusOK, fullModel)
}

// [Note]
// No RequestBody required for "DELETE /softwaremodel/target/{id}"

// [Note]
// No ResponseBody required for "DELETE /softwaremodel/target/{id}"

// DeleteTargetSoftwareModel godoc
// @ID DeleteTargetSoftwareModel
// @Summary Delete a target software user model
// @Description Delete a target software user model with the given information.
// @Tags [API] Target Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {string} string "Successfully Deleted the Target Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /softwaremodel/target/{id} [delete]
func DeleteTargetSoftwareModel(c echo.Context) error {
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
			if isSoftwareModel, exists := model["isSoftwareModel"]; exists {
				if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok && isSoftwareModelBool {
					log.Info().Msg("This model is a Software Model!!")
				} else {
					msg := "The Given ID is Not a Software Model ID"
					log.Error().Msgf("%s : [%s]", msg, c.Param("id"))
					newErr := fmt.Errorf("%s : [%s]", msg, c.Param("id"))
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isSoftwareModel' does not exist"
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

// ##############################################################################################
// ### Unified Software Migration User Model (Source + Target)
// ##############################################################################################

// [Note]
// CreateSoftwareModelReq is a unified request body for creating or updating either a source
// or a target software migration user model. Only the field matching the 'isTargetModel'
// query parameter needs to be provided (sourceSoftwareModel or targetSoftwareModel).
type CreateSoftwareModelReq struct {
	UserId              string                                      `json:"userId"`
	NodeId              string                                      `json:"nodeId,omitempty"`
	IsInitUserModel     bool                                        `json:"isInitUserModel"`
	UserModelName       string                                      `json:"userModelName"`
	UserModelVer        string                                      `json:"userModelVersion"`
	Description         string                                      `json:"description"`
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

// sourceSoftwareModelRecord and targetSoftwareModelRecord are the stored forms of software models whose
// 'sourceSoftwareModel' / 'targetSoftwareModel' follow the struct of their own smdl version.
type sourceSoftwareModelRecord struct {
	SourceSoftwareModelRespInfo
	SourceSoftwareModel json.RawMessage `json:"sourceSoftwareModel"`
}

type targetSoftwareModelRecord struct {
	TargetSoftwareModelRespInfo
	TargetSoftwareModel json.RawMessage `json:"targetSoftwareModel"`
}

// rawSoftwareModel returns the raw software model field matching 'isTargetModel'.
func (r *softwareModelRawReq) rawSoftwareModel(isTargetModel bool) json.RawMessage {
	if isTargetModel {
		return r.TargetSoftwareModel
	}
	return r.SourceSoftwareModel
}

func softwareModelKind(isTargetModel bool) string {
	if isTargetModel {
		return "target"
	}
	return "source"
}

func softwareModelErrResp(c echo.Context, status int, err error) error {
	if status >= http.StatusInternalServerError {
		log.Error().Msg(err.Error())
	} else {
		log.Warn().Msg(err.Error())
	}
	return c.JSON(status, model.Response{Success: false, Text: err.Error()})
}

// parseIsTargetModelParam parses the required 'isTargetModel' query parameter ('true' or 'false').
func parseIsTargetModelParam(c echo.Context) (bool, error) {
	param := c.QueryParam("isTargetModel")
	if strings.EqualFold(param, "true") {
		return true, nil
	}
	if strings.EqualFold(param, "false") {
		return false, nil
	}
	return false, fmt.Errorf("invalid request: 'isTargetModel' must be 'true' or 'false', got '%s'", param)
}

// getSoftwareModelFromStore gets the software model with the given ID and verifies that it is
// a software model of the kind (source or target) given by 'isTargetModel'.
// On failure, it also returns the HTTP status code to respond with.
func getSoftwareModelFromStore(id string, isTargetModel bool) (map[string]interface{}, int, error) {
	stored, exists := lkvstore.Get(id)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("failed to find the model from db with id: [%s]", id)
	}

	m, ok := stored.(map[string]interface{})
	if !ok {
		return nil, http.StatusInternalServerError, fmt.Errorf("internal error: unexpected model data format")
	}

	if isSoftwareModel, ok := m["isSoftwareModel"].(bool); !ok || !isSoftwareModel {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid request: the model with id [%s] is not a software model", id)
	}

	storedIsTargetModel, ok := m["isTargetModel"].(bool)
	if !ok {
		return nil, http.StatusBadRequest, fmt.Errorf("'isTargetModel' of the model with id [%s] does not exist or is not a boolean type", id)
	}
	if storedIsTargetModel != isTargetModel {
		return nil, http.StatusBadRequest, fmt.Errorf("model type mismatch: the software model with id [%s] is a '%s' model, not '%s'",
			id, softwareModelKind(storedIsTargetModel), softwareModelKind(isTargetModel))
	}

	return m, http.StatusOK, nil
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
// @Summary Get a list of software migration user models (source or target)
// @Description Get a list of software migration user models. Use 'isTargetModel' to select source or target models.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param isTargetModel query string true "Whether to retrieve target models (true) or source models (false)" Enums(true, false)
// @Success 200 {array}  map[string]interface{} "Successfully obtained software migration user models"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 500 {object} model.Response
// @Router /software-model [get]
func GetSoftwareModels(c echo.Context) error {
	isTargetModel, err := parseIsTargetModelParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# GetSoftwareModels: isTargetModel=[%v]", isTargetModel)

	result := []map[string]interface{}{}
	modelList, exists := lkvstore.GetWithPrefix("")
	if !exists {
		return c.JSON(http.StatusOK, result)
	}

	for _, item := range modelList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if isSoftwareModel, ok := m["isSoftwareModel"].(bool); !ok || !isSoftwareModel {
			continue
		}
		if storedIsTargetModel, ok := m["isTargetModel"].(bool); !ok || storedIsTargetModel != isTargetModel {
			continue
		}
		result = append(result, m)
	}

	return c.JSON(http.StatusOK, result)
}

// GetSoftwareModel godoc
// @ID GetSoftwareModel
// @Summary Get a specific software migration user model (source or target)
// @Description Get a specific software migration user model by ID. Use 'isTargetModel' to specify whether it is a source or target model.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param isTargetModel query string true "Whether the model is a target model (true) or a source model (false)" Enums(true, false)
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

	isTargetModel, err := parseIsTargetModelParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# GetSoftwareModel: id=[%s], isTargetModel=[%v]", id, isTargetModel)

	m, status, err := getSoftwareModelFromStore(id, isTargetModel)
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	return c.JSON(http.StatusOK, m)
}

// CreateSoftwareModel godoc
// @ID CreateSoftwareModel
// @Summary Create a new software migration user model (source or target)
// @Description Create a new software migration user model. Use 'isTargetModel' to select source or target model.
// @Description Provide 'sourceSoftwareModel' for a source model (isTargetModel=false), or 'targetSoftwareModel' for a target model (isTargetModel=true).
// @Description The 'sourceSoftwareModel' (or 'targetSoftwareModel') is validated against the 'SourceGroupSoftwareProperty' (or 'TargetGroupSoftwareProperty') struct of the cm-grasshopper/smdl tagged version given by 'softwareModelVersion', and is stored in that struct's form.
// @Description 'nodeId' is an optional identifier of the node the model is associated with.
// @Description (The body schema below shows the struct of the cm-grasshopper/smdl version built into cm-damselfly. To see the struct of another version, choose 'smdl/{version}/doc.json' in 'Select a definition' at the top of Swagger UI.)
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param isTargetModel query string true "Whether to create a target model (true) or a source model (false)" Enums(true, false)
// @Param softwareModelVersion query string false "Software model version (a cm-grasshopper 'smdl/vX.Y.Z' tag, e.g. v0.1.3). If empty, the latest tag is used. See 'GET /model/version' for available versions."
// @Param Model body CreateSoftwareModelReq true "Software model information"
// @Success 201 {object} object "Successfully created the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 500 {object} model.Response
// @Router /software-model [post]
func CreateSoftwareModel(c echo.Context) error {
	isTargetModel, err := parseIsTargetModelParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}

	reqBody := new(softwareModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	requestedVer := strings.TrimSpace(c.QueryParam("softwareModelVersion"))
	log.Info().Msgf("# CreateSoftwareModel: isTargetModel=[%v], softwareModelVersion=[%s]", isTargetModel, requestedVer)

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

	softwareModel, status, err := normalizeSoftwareModel(c.Request().Context(), resultVer, isTargetModel, reqBody.rawSoftwareModel(isTargetModel))
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	id := uuid.New().String()
	log.Info().Msgf("Generated UUID : [%s]", id)

	createTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	var userModel interface{}
	if isTargetModel {
		userModel = targetSoftwareModelRecord{TargetSoftwareModelRespInfo: TargetSoftwareModelRespInfo{
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
			IsTargetModel:    true,
			ModelType:        SWModel,
		}, TargetSoftwareModel: softwareModel}
	} else {
		userModel = sourceSoftwareModelRecord{SourceSoftwareModelRespInfo: SourceSoftwareModelRespInfo{
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
			IsTargetModel:    false,
			ModelType:        SWModel,
		}, SourceSoftwareModel: softwareModel}
	}

	if err := saveSoftwareModelToStore(id, userModel); err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusCreated, userModel)
}

// UpdateSoftwareModel godoc
// @ID UpdateSoftwareModel
// @Summary Update a specific software migration user model (source or target)
// @Description Update a specific software migration user model by ID. Use 'isTargetModel' to specify whether it is a source or target model.
// @Description Provide 'sourceSoftwareModel' for a source model (isTargetModel=false), or 'targetSoftwareModel' for a target model (isTargetModel=true).
// @Description 'createTime' and 'softwareModelVersion' of the stored model are preserved, and 'sourceSoftwareModel' (or 'targetSoftwareModel') is validated against the struct of that cm-grasshopper/smdl tagged version.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param isTargetModel query string true "Whether the model is a target model (true) or a source model (false)" Enums(true, false)
// @Param Model body CreateSoftwareModelReq true "Software model information to update"
// @Success 200 {object} object "Successfully updated the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /software-model/{id} [put]
func UpdateSoftwareModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	isTargetModel, err := parseIsTargetModelParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}

	reqBody := new(softwareModelRawReq)
	if err := c.Bind(reqBody); err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request : [%v]", err))
	}
	log.Info().Msgf("# UpdateSoftwareModel: id=[%s], isTargetModel=[%v]", id, isTargetModel)

	existing, status, err := getSoftwareModelFromStore(id, isTargetModel)
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
	rawSoftwareModel := reqBody.rawSoftwareModel(isTargetModel)
	if _, verErr := selectModelVersion(preservedVer, softwareVersions); preservedVer != "" && verErr == nil {
		softwareModel, status, err = normalizeSoftwareModel(c.Request().Context(), preservedVer, isTargetModel, rawSoftwareModel)
	} else {
		log.Warn().Msgf("The stored model version [%s] is not a cm-grasshopper/smdl tag. Using the built-in model struct.", preservedVer)
		softwareModel, status, err = normalizeSoftwareModelWithCompiledStruct(isTargetModel, rawSoftwareModel)
	}
	if err != nil {
		return softwareModelErrResp(c, status, err)
	}

	updateTime, err := getSeoulCurrentTime()
	if err != nil {
		log.Debug().Msg("Failed to Get the Current time!!")
	}

	// The existing model is loaded first so that the fields not given in the request
	// (e.g. 'createTime', 'softwareModelVersion') are preserved.
	var updatedModel interface{}
	if isTargetModel {
		fullModel := new(targetSoftwareModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.Id = id
		fullModel.UserId = reqBody.UserId
		fullModel.NodeId = reqBody.NodeId
		fullModel.IsInitUserModel = reqBody.IsInitUserModel
		fullModel.UserModelName = reqBody.UserModelName
		fullModel.UserModelVer = reqBody.UserModelVer
		fullModel.Description = reqBody.Description
		fullModel.UpdateTime = updateTime
		fullModel.IsSoftwareModel = true
		fullModel.IsTargetModel = true
		fullModel.ModelType = SWModel
		fullModel.TargetSoftwareModel = softwareModel
		updatedModel = fullModel
	} else {
		fullModel := new(sourceSoftwareModelRecord)
		if err := json.Unmarshal(existingBytes, fullModel); err != nil {
			return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal the existing model : [%v]", err))
		}
		fullModel.Id = id
		fullModel.UserId = reqBody.UserId
		fullModel.NodeId = reqBody.NodeId
		fullModel.IsInitUserModel = reqBody.IsInitUserModel
		fullModel.UserModelName = reqBody.UserModelName
		fullModel.UserModelVer = reqBody.UserModelVer
		fullModel.Description = reqBody.Description
		fullModel.UpdateTime = updateTime
		fullModel.IsSoftwareModel = true
		fullModel.IsTargetModel = false
		fullModel.ModelType = SWModel
		fullModel.SourceSoftwareModel = softwareModel
		updatedModel = fullModel
	}

	if err := saveSoftwareModelToStore(id, updatedModel); err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, err)
	}
	log.Info().Msgf("Successfully updated the model: [%s]", id)

	return c.JSON(http.StatusOK, updatedModel)
}

// DeleteSoftwareModel godoc
// @ID DeleteSoftwareModel
// @Summary Delete a specific software migration user model (source or target)
// @Description Delete a specific software migration user model by ID. Use 'isTargetModel' to specify whether it is a source or target model.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param isTargetModel query string true "Whether the model is a target model (true) or a source model (false)" Enums(true, false)
// @Success 200 {string} string "Successfully deleted the software migration user model"
// @Failure 400 {object} model.Response "Invalid request parameter"
// @Failure 404 {object} model.Response "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /software-model/{id} [delete]
func DeleteSoftwareModel(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return softwareModelErrResp(c, http.StatusBadRequest, fmt.Errorf("invalid request: model id is required"))
	}

	isTargetModel, err := parseIsTargetModelParam(c)
	if err != nil {
		return softwareModelErrResp(c, http.StatusBadRequest, err)
	}
	log.Info().Msgf("# DeleteSoftwareModel: id=[%s], isTargetModel=[%v]", id, isTargetModel)

	if _, status, err := getSoftwareModelFromStore(id, isTargetModel); err != nil {
		return softwareModelErrResp(c, status, err)
	}

	lkvstore.Delete(id)
	log.Info().Msgf("Succeeded in Deleting the model : [%s]", id)

	if err := lkvstore.SaveLkvStore(); err != nil {
		return softwareModelErrResp(c, http.StatusInternalServerError, fmt.Errorf("failed to save the lkvstore to file : [%v]", err))
	}
	log.Info().Msg("Succeeded in Saving the lkvstore to file.")

	return c.JSON(http.StatusOK, "Succeeded in Deleting the software model")
}
