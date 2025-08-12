package handler

import (
	"fmt"
	"net/http"
	"errors"
	"strings"
	"github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/rs/zerolog/log"

	model 			"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	softwaremodel 	"github.com/cloud-barista/cm-model/sw"
)

// ##############################################################################################
// ### Software Migration User Model
// ##############################################################################################

type SoftwareModelReqInfo struct {
	UserId          	string                  		`json:"userId"`
	IsTargetModel   	bool                    		`json:"isTargetModel"`
	IsInitUserModel 	bool                    		`json:"isInitUserModel"`
	UserModelName   	string                  		`json:"userModelName"`
	UserModelVer    	string                  		`json:"userModelVersion"`
	Description     	string                  		`json:"description"`
	SoftwareModel   	softwaremodel.SourceGroupSoftwareProperty	`json:"softwareModel" validate:"required"`	
}

type SoftwareModelRespInfo struct {
	Id              	string                  		`json:"id"`
	UserId          	string                  		`json:"userId"`
	IsTargetModel   	bool                    		`json:"isTargetModel"`
	IsInitUserModel 	bool                   			`json:"isInitUserModel"`
	UserModelName   	string                  		`json:"userModelName"`
	UserModelVer    	string                  		`json:"userModelVersion"`
	Description     	string                  		`json:"description"`
	SoftwareModelVer 	string                  		`json:"softwareModelVersion"`	
	CreateTime      	string                  		`json:"createTime"`
	UpdateTime      	string                  		`json:"updateTime"`
	IsSoftwareModel     bool                    		`json:"isSoftwareModel"`
	SoftwareModel   	softwaremodel.SourceGroupSoftwareProperty	`json:"softwareModel" validate:"required"`	
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetSoftwareModelsResp struct {
	Models []SoftwareModelRespInfo `json:"models"`
}

// GetSoftwareModels godoc
// @Summary Get a list of software user models
// @Description Get a list of software user models.
// @Tags [API] Software Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetSoftwareModelsResp "Successfully Obtained Software Migration User Models"
// @Failure 404 {object} model.Response
// @Router /softwaremodel [get]
func GetSoftwareModels(c echo.Context) error {
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		//  Returns Only Software Models
		var softwareModels []map[string]interface{}
		for _, model := range modelList {
			// fmt.Printf("# Model value : %v", model)
			if model, ok := model.(map[string]interface{}); ok {
				if isSoftwareModel, exists := model["isSoftwareModel"]; exists && isSoftwareModel == true {
					softwareModels = append(softwareModels, model)
				}
			}
		}

		if len(softwareModels) < 1 {
			msg := "Failed to Find Any Model"
			log.Debug().Msg(msg)
			newErr := errors.New(msg)
			return c.JSON(http.StatusNotFound, newErr)
		}

		return c.JSON(http.StatusOK, softwareModels)
	} else {
		msg := "Failed to Find Any Model from DB"
		log.Debug().Msg(msg)		// Not log.Error()
		newErr := errors.New(msg)
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type GetSoftwareModelResp struct {
	SoftwareModelRespInfo
}

// GetSoftwareModel godoc
// @Summary Get a specific software user model
// @Description Get a specific software user model.
// @Tags [API] Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetSoftwareModelResp "Successfully Obtained the Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /softwaremodel/{id} [get]
func GetSoftwareModel(c echo.Context) error {
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
type CreateSoftwareModelReq struct {
	SoftwareModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of SoftwareModel
type CreateSoftwareModelResp struct {
	SoftwareModelRespInfo
}

// CreateSoftwareModel godoc
// @Summary Create a new software user model
// @Description Create a new software user model with the given information.
// @Tags [API] Software Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateSoftwareModelReq true "model information"
// @Success 201 {object} CreateSoftwareModelResp "Successfully Created the Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Router /softwaremodel [post]
func CreateSoftwareModel(c echo.Context) error {
	model := new(CreateSoftwareModelResp)

	if err := c.Bind(model); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	// fmt.Println("### CreateSoftwareModelResp",)
	// spew.Dump(model)

	randomStr, err := generateRandomString(15)
	if err != nil {
		msg := "Failed to Generate a random string!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msgf("Random 15-length of string : [%s]", randomStr)
	}
	model.Id = randomStr

	time, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	}
	model.CreateTime = time
	model.IsSoftwareModel = true

	softwareModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
	if err != nil {
		msg := "Failed to Get the Module Verion!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msgf("Software Model version: %s", softwareModelVer)
	}
	model.SoftwareModelVer = softwareModelVer

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
// Struct Embedding is used to inherit the fields of SoftwareModel
type UpdateSoftwareModelReq struct {
	SoftwareModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of SoftwareModel
type UpdateSoftwareModelResp struct {
	SoftwareModelRespInfo
}

// UpdateSoftwareModel godoc
// @Summary Update a software user model
// @Description Update a software user model with the given information.
// @Tags [API] Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateSoftwareModelReq true "Model information to update"
// @Success 201 {object} UpdateSoftwareModelResp "Successfully Updated the Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /softwaremodel/{id} [put]
func UpdateSoftwareModel(c echo.Context) error {
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

	updateModel := new(UpdateSoftwareModelResp)

	if err := c.Bind(updateModel); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)


		err := fmt.Errorf("invalid request")
		log.Warn().Msg(err.Error())
		res := model.Response{
			Success: false,
			Text:    err.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	// fmt.Printf("New Req Values for [%s]: %v", c.Param("id"), updateModel)

	model, exists := lkvstore.Get(reqId)
	if exists {
		log.Info().Msgf("Succeeded in Finding the model : [%s]", reqId)
		// fmt.Printf("Values from DB [%s]: %v", c.Param("id"), model)

		if softwareModel, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isSoftwareModel, exists := softwareModel["isSoftwareModel"]; exists {
				if isSoftwareModelBool, ok := isSoftwareModel.(bool); ok {
					log.Info().Msgf("The value of isSoftwareModel is: %v", isSoftwareModel)

					if isSoftwareModelBool {
						log.Info().Msg("This model is a Software Model!!")
					} else {
						msg := "The Given ID is Not a Software Model ID"
						log.Error().Msgf("%s : [%s]", msg, reqId)
						newErr := fmt.Errorf("%s : [%s]", msg, reqId)
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
		
					msg := "'isSoftwareModel' is not a boolean type"
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

		if model, ok := model.(map[string]interface{}); ok {
			if softwareModelVer, exists := model["softwareModelVersion"]; exists {
				if softwareModelVerStr, ok := softwareModelVer.(string); ok {

					updateModel.SoftwareModelVer = softwareModelVerStr

					log.Info().Msgf("# softwareModelVer : [%s]", softwareModelVerStr)
				} else {
					log.Info().Msg("'softwareModelVersion' is not a string type of value")
				}
			} else {
				msg := "'softwareModelVersion' does not exist"
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

		// softwareModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
		// if err != nil {
		// 	fmt.Println("Error:", err)
		// } else {
		// 	fmt.Printf("Software Model version: %s", softwareModelVer)
		// }
		// updateModel.SoftwareModelVer = softwareModelVer

		updateModel.Id = reqId
		time, err := getSeoulCurrentTime()
		if err != nil {
			msg := "Failed to Get the Current time!!"
			log.Debug().Msg(msg)
			// newErr := errors.New(msg)
			// return c.JSON(http.StatusNotFound, newErr)
		}
		updateModel.UpdateTime = time
		updateModel.IsSoftwareModel = true

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
// No RequestBody required for "DELETE /softwaremodel/{id}"

// [Note]
// No ResponseBody required for "DELETE /softwaremodel/{id}"

// DeleteSoftwareModel godoc
// @Summary Delete a software user model
// @Description Delete a software user model with the given information.
// @Tags [API] Software Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {string} string "Successfully Deleted the Software Migration User Model"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Failure 500 {object} model.Response
// @Router /softwaremodel/{id} [delete]
func DeleteSoftwareModel(c echo.Context) error {
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
