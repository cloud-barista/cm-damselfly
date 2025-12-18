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
	softwaremodel 	"github.com/cloud-barista/cm-model/sw"
)

type Release struct {
    TagName string `json:"tag_name"`
    Name    string `json:"name"`
}

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

	var resultVer string
	modelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
	if err != nil {
		msg := "Failed to Get the Module Verion!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	} else {
		if len(modelVer) > 10 {
			release, err := getLatestRelease("cloud-barista", "cm-model")
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return err
			}    
			log.Info().Msgf("Latest version: %s\n", release.TagName)
			// log.Info().Msgf("Release name: %s\n", release.Name)
			resultVer = release.TagName
		} else {
			resultVer = modelVer
		}
		log.Info().Msgf("Software Model version: %s", resultVer)
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

	var resultVer string
	modelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
	if err != nil {
		msg := "Failed to Get the Module Verion!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	} else {
		if len(modelVer) > 10 {
			release, err := getLatestRelease("cloud-barista", "cm-model")
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return err
			}    
			log.Info().Msgf("Latest version: %s\n", release.TagName)
			// log.Info().Msgf("Release name: %s\n", release.Name)
			resultVer = release.TagName
		} else {
			resultVer = modelVer
		}
		log.Info().Msgf("Software Model version: %s", resultVer)
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
