package handler

import (
	"fmt"
	"net/http"
	"errors"
	"strconv"
	"strings"
	"github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/rs/zerolog/log"

	model "github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"

	onpreminfra "github.com/cloud-barista/cm-model/infra/onprem"
	cloudinfra  "github.com/cloud-barista/cm-model/infra/cloud"
	software    "github.com/cloud-barista/cm-model/sw"
)

// ##############################################################################################
// ### On-premise and Cloud Migration User Model
// ##############################################################################################

type ModelRespInfo struct {
	Id               string                  `json:"id"`
	UserId           string                  `json:"userId"`
	IsInitUserModel  bool                    `json:"isInitUserModel"`
	UserModelName    string                  `json:"userModelName"`
	Description      string                  `json:"description"`
	UserModelVer     string                  `json:"userModelVersion"`
	CreateTime       string                  `json:"createTime"`
	UpdateTime       string                  `json:"updateTime"`
	IsTargetModel    bool                    `json:"isTargetModel"`
	IsCloudModel     bool                    `json:"isCloudModel"`
	OnPremModelVer   string                  `json:"onpremModelVersion"`
	CloudModelVer    string                  `json:"cloudModelVersion"`
	CSP              string                  `json:"csp"`
	Region           string                  `json:"region"`
	Zone             string                  `json:"zone"`
	OnpremInfraModel onpreminfra.OnpremInfra `json:"onpremiseInfraModel" validate:"required"`
	CloudInfraModel  cloudinfra.CloudInfra 	 `json:"cloudInfraModel" validate:"required"`
	SoftwareModel    software.Software	 	 `json:"softwareModel" validate:"required"`	
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetModelsResp struct {
	Models []ModelRespInfo `json:"models"`
}

// GetModels godoc
// @Summary Get a list of all user models
// @Description Get a list of all user models.
// @Tags [API] Migration User Models
// @Accept  json
// @Produce  json
// @Param isTargetModel path bool true "Is TargetModel ?"
// @Success 200 {object} GetModelsResp "Successfully Obtained Migration User Models."
// @Failure 404 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /model/{isTargetModel} [get]
func GetModels(c echo.Context) error {
	param := c.Param("isTargetModel")
	// fmt.Printf("# The value of 'isTargetModel' parameter : [%v]", isTargetModel)

	if strings.EqualFold(param, "true") || strings.EqualFold(param, "false") {
		// if strings.EqualFold(param, "true") {
		// 	fmt.Printf("# Models to Get : Target models")
		// } else {
		// 	fmt.Printf("# Models to Get : Source models")
		// }
	} else {
		newErr := fmt.Errorf("invalid request, invalid type of parameter")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	// Convert the string to a boolean
	isTargetmodel, err := strconv.ParseBool(param)
	if err != nil {
		newErr := fmt.Errorf("invalid request : [%v]", err)
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	var models []map[string]interface{}
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		if isTargetmodel { // Only Tareget models
			for _, model := range modelList {
				if model, ok := model.(map[string]interface{}); ok {
					if isTargetModel, exists := model["isTargetModel"]; exists && isTargetModel == true {
						models = append(models, model)
					}
				}
			}
		} else { // Only Source models
			for _, model := range modelList {
				if model, ok := model.(map[string]interface{}); ok {
					if isTargetModel, exists := model["isTargetModel"]; exists && isTargetModel == false {
						models = append(models, model)
					}
				}
			}
		}
		return c.JSON(http.StatusOK, models)
	} else {
		return c.JSON(http.StatusOK, nil)
	}
}

type ModelsVersionRespInfo struct {
	OnPremModelVer string `json:"onpremModelVersion"`
	CloudModelVer  string `json:"cloudModelVersion"`
}

type GetModelsVersionResp struct {
	ModelsVersion ModelsVersionRespInfo `json:"modelsVersion"`
}

// GetModelsVersion godoc
// @Summary Get the versions of all models(schemata of on-premise/cloud migration models)
// @Description Get the versions of all models(schemata of on-premise/cloud migration models)
// @Tags [API] Migration Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetModelsVersionResp "This is the versions of all models(schemata)"
// @Failure 404 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /model/version [get]
func GetModelsVersion(c echo.Context) error {

	modelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
	if err != nil {
		newErr := fmt.Errorf("failed to get the 'cm-model' module version : [%v]", err)
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusInternalServerError, res)
	}

	// cloudModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
	// if err != nil {
	// 	newErr := fmt.Errorf("failed to get the 'cb-tumblebug' module version : [%v]", err)
	// 	log.Error().Msg(newErr.Error())
	// 	res := model.Response{
	// 		Success: false,
	// 		Text:    newErr.Error(),
	// 	}
	// 	return c.JSON(http.StatusInternalServerError, res)
	// }

	modelsVersionInfo := ModelsVersionRespInfo{
		OnPremModelVer: modelVer,
		// CloudModelVer:  cloudModelVer,
	}
	res := GetModelsVersionResp{
		ModelsVersion: modelsVersionInfo,
	}
	return c.JSON(http.StatusOK, res)
}

// ##############################################################################################
// ### On-premise Migration User Model
// ##############################################################################################

type OnPremModelReqInfo struct {
	UserId           string             `json:"userId"`
	IsInitUserModel  bool               `json:"isInitUserModel"`
	UserModelName    string             `json:"userModelName"`
	Description      string             `json:"description"`
	UserModelVer     string             `json:"userModelVersion"`
	OnpremInfraModel onpreminfra.OnpremInfra `json:"onpremiseInfraModel" validate:"required"`
	SoftwareModel    software.Software	 	 `json:"softwareModel" validate:"required"`	
}

type OnPremModelRespInfo struct {
	Id               string             `json:"id"`
	UserId           string             `json:"userId"`
	IsInitUserModel  bool               `json:"isInitUserModel"`
	UserModelName    string             `json:"userModelName"`
	Description      string             `json:"description"`
	UserModelVer     string             `json:"userModelVersion"`
	OnPremModelVer   string             `json:"onpremModelVersion"`
	CreateTime       string             `json:"createTime"`
	UpdateTime       string             `json:"updateTime"`
	IsTargetModel    bool               `json:"isTargetModel"`
	IsCloudModel     bool               `json:"isCloudModel"`
	OnpremInfraModel onpreminfra.OnpremInfra `json:"onpremiseInfraModel" validate:"required"`
	SoftwareModel    software.Software	 	 `json:"softwareModel" validate:"required"`	
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetOnPremModelsResp struct {
	Models []OnPremModelRespInfo `json:"models"`
}

// GetOnPremModels godoc
// @Summary Get a list of on-premise models
// @Description Get a list of on-premise models.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetOnPremModelsResp "Successfully Obtained On-Premise Migration User Models"
// @Failure 404 {object} model.Response
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
// @Summary Get a specific on-premise model
// @Description Get a specific on-premise model.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetOnPremModelResp "Successfully Obtained the On-Premise Migration User Model"
// @Failure 404 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /onpremmodel/{id} [get]
func GetOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		newErr := fmt.Errorf("invalid request, invalid model id")
		log.Warn().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusBadRequest, res)
	}
	log.Info().Msgf("# Model ID to Get : [%s]", c.Param("id"))

	/*
		// GetWithPrefix returns the values for a given key prefix.
		modelList, exists := lkvstore.GetWithPrefix("")
		if exists {
			// # Returns Only On-prem Models
			var onpremModel map[string]interface{}
			for _, model := range modelList {
				if model, ok := model.(map[string]interface{}); ok {

					if isCloudModel, exists := model["isCloudModel"]; exists && isCloudModel == false {

						if id, exists := model["id"]; exists {
							// fmt.Printf("Loaded value-1 for [%s]: %v", c.Param("id"), model)
							if id, ok := id.(string); ok {
								if id == c.Param("id") {
									// fmt.Printf("Loaded value-2 for [%s]: %v", c.Param("id"), model)
									onpremModel = model
									return c.JSON(http.StatusOK, onpremModel)

									// 			if isCloudModelBool {
									// 				newErr := fmt.Errorf("The Given ID is Not a On-premise Model ID : [%s]", c.Param("id"))
									// 				return c.JSON(http.StatusNotFound, newErr)
									// 			} else {
									// 				msg := "This model is a On-premise Model!!"
									// 				log.Error().Msg(msg)

									// 				newErr := fmt.Errorf(msg)
									// 				return c.JSON(http.StatusNotFound, newErr)
									// 			}

								}
							} else {
								msg := ("'id' is not a string type")
								log.Error().Msg(msg)

								newErr := fmt.Errorf("'id' is not a string type")
								return c.JSON(http.StatusNotFound, newErr)
							}
						} else {
							msg := ("'id' does not exist")
							log.Error().Msg(msg)

							newErr := fmt.Errorf("'id' does not exist")
							return c.JSON(http.StatusNotFound, newErr)
						}
					}
					// else {
					// 	msg := ("'id' does not exist")
					// 	log.Error().Msg(msg)

					// 	newErr := fmt.Errorf("'isCloudModel' does not exist")
					// 	return c.JSON(http.StatusNotFound, newErr)
					// }

					// fmt.Printf("Loaded value-1 for [%s]: %v", c.Param("id"), model)

					// if Id, exists := model["id"]; exists && Id == c.Param("id") {

					// 	fmt.Printf("Loaded value-2 for [%s]: %v", c.Param("id"), model)

					// 	if isCloudModel, exists := model["isCloudModel"]; exists {
					// 		if isCloudModelBool, ok := isCloudModel.(bool); ok {
					// 			if isCloudModelBool {
					// 				newErr := fmt.Errorf("The Given ID is Not a On-premise Model ID : [%s]", c.Param("id"))
					// 				return c.JSON(http.StatusNotFound, newErr)
					// 			} else {
					// 				msg := "This model is a On-premise Model!!"
					// 				log.Error().Msg(msg)

					// 				newErr := fmt.Errorf(msg)
					// 				return c.JSON(http.StatusNotFound, newErr)
					// 			}
					// 		} else {
					// 			msg := ("'isCloudModel' is not a boolean type")
					// 			log.Error().Msg(msg)

					// 			newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
					// 			return c.JSON(http.StatusNotFound, newErr)
					// 		}
					// 	} else {
					// 		msg := ("'isCloudModel' does not exist")
					// 		log.Error().Msg(msg)

					// 		newErr := fmt.Errorf("'isCloudModel' does not exist")
					// 		return c.JSON(http.StatusNotFound, newErr)
					// 	}
					// } else {
					// 	onpremModel = model
					// 	return c.JSON(http.StatusOK, onpremModel)
					// }
				}
			}
			// return c.JSON(http.StatusOK, onpremModel)

			newErr := fmt.Errorf("Failed to Find the Model : [%s]", c.Param("id"))
			return c.JSON(http.StatusNotFound, newErr)

		} else {
			newErr := fmt.Errorf("Failed to Find the Model : [%s]", c.Param("id"))
			return c.JSON(http.StatusNotFound, newErr)
		}
	*/

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
				newErr := fmt.Errorf("'isCloudModel' does not exist")
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
		newErr := fmt.Errorf("failed to find the model from db with the id")
		log.Error().Msg(newErr.Error())
		res := model.Response{
			Success: false,
			Text:    newErr.Error(),
		}
		return c.JSON(http.StatusNotFound, res)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type CreateOnPremModelReq struct {
	OnPremModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type CreateOnPremModelResp struct {
	OnPremModelRespInfo
}

// CreateOnPremModel godoc
// @Summary Create a new on-premise model
// @Description Create a new on-premise model with the given information.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateOnPremModelReq true "model information"
// @Success 201 {object} CreateOnPremModelResp "Successfully Created the On-Premise Migration User Model"
// @Failure 404 {object} model.Response
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

	randomStr, err := generateRandomString(15)
	if err != nil {
		msg := "Failed to Generate a random string!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msgf("Random 15-length of string : [%s]", randomStr)
	}
	userModel.Id = randomStr

	time, err := getSeoulCurrentTime()
	if err != nil {
		msg := "Failed to Get the Current time!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	}
	userModel.CreateTime = time
	userModel.IsTargetModel = false
	userModel.IsCloudModel = false

	onpremModelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
	if err != nil {
		msg := "Failed to Get the Module Verion!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msgf("On-premise Model version: [%s]", onpremModelVer)
	}
	userModel.OnPremModelVer = onpremModelVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the userModel to the key-value store
	lkvstore.Put(randomStr, userModel)

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
// Struct Embedding is used to inherit the fields of MyOnPremModel
type UpdateOnPremModelReq struct {
	OnPremModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type UpdateOnPremModelResp struct {
	OnPremModelRespInfo
}

// UpdateOnPremModel godoc
// @Summary Update a on-premise model
// @Description Update a on-premise model with the given information.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateOnPremModelReq true "Model information to update"
// @Success 201 {object} UpdateOnPremModelResp "Successfully Updated the On-Premise Migration User Model"
// @Failure 404 {object} model.Response
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
				msg := "'isCloudModel' does not exist"
				log.Error().Msg(msg)
				newErr := errors.New(msg)
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if onPremModelVer, exists := model["onpremModelVersion"]; exists {
				if onpremModelVerStr, ok := onPremModelVer.(string); ok {
					updateModel.OnPremModelVer = onpremModelVerStr
					log.Info().Msgf("# onpremModelVer : [%s]", onpremModelVerStr)
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
// @Summary Delete a on-premise model
// @Description Delete a on-premise model with the given information.
// @Tags [API] On-Premise Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {string} string "Successfully Deleted the On-Premise Migration User Model"


// @Failure 404 {object} model.Response
// @Failure 500 {object} model.Response


// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
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
				msg := "'isCloudModel' does not exist"
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
	UserId          string                  `json:"userId"`
	IsTargetModel   bool                    `json:"isTargetModel"`
	IsInitUserModel bool                    `json:"isInitUserModel"`
	UserModelName   string                  `json:"userModelName"`
	Description     string                  `json:"description"`
	UserModelVer    string                  `json:"userModelVersion"`
	CSP             string                  `json:"csp"`
	Region          string                  `json:"region"`
	Zone            string                  `json:"zone"`
	CloudInfraModel  cloudinfra.CloudInfra 	`json:"cloudInfraModel" validate:"required"`
	SoftwareModel    software.Software	    `json:"softwareModel" validate:"required"`	
}

type CloudModelRespInfo struct {
	Id              string                  `json:"id"`
	UserId          string                  `json:"userId"`
	IsTargetModel   bool                    `json:"isTargetModel"`
	IsInitUserModel bool                    `json:"isInitUserModel"`
	UserModelName   string                  `json:"userModelName"`
	Description     string                  `json:"description"`
	UserModelVer    string                  `json:"userModelVersion"`
	CreateTime      string                  `json:"createTime"`
	UpdateTime      string                  `json:"updateTime"`
	CSP             string                  `json:"csp"`
	Region          string                  `json:"region"`
	Zone            string                  `json:"zone"`
	IsCloudModel    bool                    `json:"isCloudModel"`
	CloudModelVer   string                  `json:"cloudModelVersion"`
	CloudInfraModel  cloudinfra.CloudInfra 	`json:"cloudInfraModel" validate:"required"`
	SoftwareModel    software.Software	 	`json:"softwareModel" validate:"required"`	
}

// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetCloudModelsResp struct {
	Models []CloudModelRespInfo `json:"models"`
}

// GetCloudModels godoc
// @Summary Get a list of cloud models
// @Description Get a list of cloud models.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetCloudModelsResp "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
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
			msg := "Failed to Find Any Model"
			log.Debug().Msg(msg)
			newErr := errors.New(msg)
			return c.JSON(http.StatusNotFound, newErr)
		}

		return c.JSON(http.StatusOK, cloudModels)
	} else {
		msg := "Failed to Find Any Model from DB"
		log.Debug().Msg(msg)		// Not log.Error()
		newErr := errors.New(msg)
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type GetCloudModelResp struct {
	CloudModelRespInfo
}

// GetCloudModel godoc
// @Summary Get a specific cloud model
// @Description Get a specific cloud model.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 200 {object} GetCloudModelResp "(Sample) a model"
// @Failure 404 {object} object "model not found"
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
				msg := "'isCloudModel' does not exist"
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
// Struct Embedding is used to inherit the fields of MyCloudModel
type CreateCloudModelReq struct {
	CloudModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type CreateCloudModelResp struct {
	CloudModelRespInfo
}

// CreateCloudModel godoc
// @Summary Create a new cloud model
// @Description Create a new cloud model with the given information.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param Model body CreateCloudModelReq true "model information"
// @Success 201 {object} CreateCloudModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /cloudmodel [post]
func CreateCloudModel(c echo.Context) error {
	model := new(CreateCloudModelResp)

	if err := c.Bind(model); err != nil {
		msg := "Invalid Request!!"
		log.Error().Msg(msg)
		newErr := errors.New(msg)
		return c.JSON(http.StatusBadRequest, newErr)
	}
	// fmt.Println("### CreateCloudModelResp",)
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
	model.IsCloudModel = true

	cloudModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
	if err != nil {
		msg := "Failed to Get the Module Verion!!"
		log.Debug().Msg(msg)
		// newErr := errors.New(msg)
		// return c.JSON(http.StatusNotFound, newErr)
	} else {
		log.Info().Msgf("Cloud Model version: %s", cloudModelVer)
	}
	model.CloudModelVer = cloudModelVer

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
// Struct Embedding is used to inherit the fields of MyCloudModel
type UpdateCloudModelReq struct {
	CloudModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type UpdateCloudModelResp struct {
	CloudModelRespInfo
}

// UpdateCloudModel godoc
// @Summary Update a cloud model
// @Description Update a cloud model with the given information.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Param Model body UpdateCloudModelReq true "Model information to update"
// @Success 201 {object} UpdateCloudModelResp "Successfully updated!!"
// @Failure 404 {object} model.Response
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
				msg := "'isCloudModel' does not exist"
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
// @Summary Delete a cloud model
// @Description Delete a cloud model with the given information.
// @Tags [API] Cloud Migration User Models
// @Accept  json
// @Produce  json
// @Param id path string true "Model ID"
// @Success 201 {object} UpdateCloudModelResp "Successfully deleted!!"
// @Failure 404 {object} model.Response
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
				msg := "'isCloudModel' does not exist"
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
