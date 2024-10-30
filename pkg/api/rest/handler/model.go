package handler

import (
	"fmt"
	"net/http"
	// "reflect"
	// "encoding/json"
	"strconv"
	"strings"
	"github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"

	tbmodel "github.com/cloud-barista/cb-tumblebug/src/core/model"
	onprem "github.com/cloud-barista/cm-model/infra/onprem"
)

// ##############################################################################################
// ### On-premise and Cloud Migration User Model
// ##############################################################################################

type ModelRespInfo struct {
	Id   					string    				`json:"id"`
	UserId 					string    				`json:"userId"`
	IsInitUserModel			bool	  				`json:"isInitUserModel"`
	UserModelName 			string  				`json:"userModelName"`
	Description 			string 					`json:"description"`
	UserModelVer			string  				`json:"userModelVersion"`	
	CreateTime				string					`json:"createTime"`
	UpdateTime				string					`json:"updateTime"`
	IsTargetModel			bool	  				`json:"isTargetModel"`
	IsCloudModel			bool					`json:"isCloudModel"`
	OnPremModelVer			string 					`json:"onpremModelVersion"`
	CloudModelVer			string 					`json:"cloudModelVersion"`
	CSP						string					`json:"csp"`
	Region					string					`json:"region"`
	Zone					string					`json:"zone"`
	OnPremInfraModel		onprem.OnPremInfra 		`json:"onpremiseInfraModel" validate:"required"`
	CloudInfraModel			tbmodel.TbMciDynamicReq `json:"cloudInfraModel" validate:"required"`
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
// @Success 200 {object} GetModelsResp "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
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
		return c.JSON(http.StatusBadRequest, "Invalid type of parameter!!")
	}

	// Convert the string to a boolean
	isTargetmodel, err := strconv.ParseBool(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid boolean value")
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

		if len(models) < 1 {			
			msg := "Failed to Find Any Model"
			log.Debug().Msg(msg)

			newErr := fmt.Errorf(msg)
			return c.JSON(http.StatusNotFound, newErr)
		}

		return c.JSON(http.StatusOK, models)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model from DB\n")
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type ModelsVersionRespInfo struct {
	OnPremModelVer			string 					`json:"onpremModelVersion"`
	CloudModelVer			string 					`json:"cloudModelVersion"`
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
// @Success 200 {object} GetModelsVersionResp "(sample) This is the versions of all models(schemata)"
// @Failure 404 {object} object "verson of models not found"
// @Router /model/version [get]
func GetModelsVersion(c echo.Context) error {

	onpremModelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
	if err != nil {
		log.Error().Msgf("Failed to Get the Module Version : [%v]", err)		
	}

	cloudModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
	if err != nil {
		log.Error().Msgf("Failed to Get the Module Version : [%v]", err)
	}

	modelsVersionInfo := ModelsVersionRespInfo{
		OnPremModelVer: onpremModelVer,
		CloudModelVer:  cloudModelVer,
	}

	response := GetModelsVersionResp{
		ModelsVersion: modelsVersionInfo,
	}

	return c.JSON(http.StatusOK, response)
}

// ##############################################################################################
// ### On-premise Migration User Model
// ##############################################################################################

type OnPremModelReqInfo struct {
	UserId 					string    			`json:"userId"`
	IsInitUserModel			bool	  			`json:"isInitUserModel"`
	UserModelName 			string  			`json:"userModelName"`
	Description 			string 				`json:"description"`
	UserModelVer			string  			`json:"userModelVersion"`
	OnPremInfraModel 		onprem.OnPremInfra 	`json:"onpremiseInfraModel" validate:"required"`
}

type OnPremModelRespInfo struct {
	Id   					string    			`json:"id"`
	UserId 					string    			`json:"userId"`
	IsInitUserModel			bool	  			`json:"isInitUserModel"`
	UserModelName 			string  			`json:"userModelName"`
	Description 			string 				`json:"description"`
	UserModelVer			string  			`json:"userModelVersion"`
	OnPremModelVer			string 				`json:"onpremModelVersion"`
	CreateTime				string				`json:"createTime"`
	UpdateTime				string				`json:"updateTime"`
	IsTargetModel			bool	  			`json:"isTargetModel"`
	IsCloudModel			bool				`json:"isCloudModel"`
	OnPremInfraModel 		onprem.OnPremInfra 	`json:"onpremiseInfraModel" validate:"required"`
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
// @Success 200 {object} GetOnPremModelsResp "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
// @Router /onpremmodel [get]
func GetOnPremModels(c echo.Context) error {
	modelList, exists := lkvstore.GetWithPrefix("")
	if exists {
		// # Returns Only On-prem Models
		var onpremModels []map[string]interface{}
		for _, model := range modelList {
			if model, ok := model.(map[string]interface{}); ok {
				if isCloudModel, exists := model["isCloudModel"]; exists && isCloudModel == false {

					// if id, exists := model["id"]; exists {
					// 	// fmt.Printf("Loaded value-1 for [%s]: %v\n", c.Param("id"), model)
					// 	if id, ok := id.(string); ok {							
					// 		log.Debug().Msgf("# Model ID to Add : [%s]\n", id)
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

		if len(onpremModels) < 1 {			
			msg := "Failed to Find Any Model"
			log.Debug().Msg(msg)

			newErr := fmt.Errorf(msg)
			return c.JSON(http.StatusNotFound, newErr)
		}

		return c.JSON(http.StatusOK, onpremModels)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
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
// @Success 200 {object} GetOnPremModelResp "(Sample) a model"
// @Failure 404 {object} object "model not found"
// @Router /onpremmodel/{id} [get]
func GetOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("# Model ID to Get : [%s]\n", c.Param("id"))

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
						// fmt.Printf("Loaded value-1 for [%s]: %v\n", c.Param("id"), model)
						if id, ok := id.(string); ok {
							if id == c.Param("id") {
								// fmt.Printf("Loaded value-2 for [%s]: %v\n", c.Param("id"), model)
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
				
				// fmt.Printf("Loaded value-1 for [%s]: %v\n", c.Param("id"), model)

				// if Id, exists := model["id"]; exists && Id == c.Param("id") {

				// 	fmt.Printf("Loaded value-2 for [%s]: %v\n", c.Param("id"), model)

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

		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)

	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
*/

	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		// fmt.Printf("Loaded value for [%s]: %v\n", c.Param("id"), value)

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						newErr := fmt.Errorf("The Given ID is Not a On-premise Model ID : [%s]", c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					} else {
						fmt.Println("This model is a On-premise Model!!")
					}
				} else {
					newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				newErr := fmt.Errorf("'isCloudModel' does not exist")
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		return c.JSON(http.StatusOK, model)
	} else {
		newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
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
// @Success 201 {object} CreateOnPremModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /onpremmodel [post]
func CreateOnPremModel(c echo.Context) error {
	model := new(CreateOnPremModelResp)

	if err := c.Bind(model); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### OnPremModel",)
	// spew.Dump(model)

    randomStr, err := generateRandomString(15)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Random 15-length of string : ", randomStr)
    }
	model.Id = randomStr

	model.CreateTime = getSeoulCurrentTime()
	model.IsTargetModel = false
	model.IsCloudModel = false

	onpremModelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("On-premise Model version: %s\n", onpremModelVer)
	}
	model.OnPremModelVer = onpremModelVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the model to the key-value store
	lkvstore.Put(randomStr, model)

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}
	
	return c.JSON(http.StatusCreated, model)
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
// @Success 201 {object} UpdateOnPremModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /onpremmodel/{id} [put]
func UpdateOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	reqId := c.Param("id")

	updateModel := new(UpdateOnPremModelResp)

	model, exists := lkvstore.Get(reqId)
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", reqId)
		fmt.Printf("### OnPrem Model ID to Update : [%s]\n", reqId)

		if err := c.Bind(updateModel); err != nil {
			return c.JSON(http.StatusBadRequest, "Invalid Request")
		}

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						log.Error().Msg("The Given ID is Not a On-premise Model ID!!")

						newErr := fmt.Errorf("The Given ID is Not a On-premise Model ID : [%s]", reqId)
						return newErr
					} else {
						fmt.Println("This model is a On-premise Model!!")
					}
				} else {
					msg := "'isCloudModel' is not a boolean type"
					log.Error().Msg(msg)

					newErr := fmt.Errorf(msg)
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'isCloudModel' does not exist"
				log.Error().Msg(msg)

				newErr := fmt.Errorf("'isCloudModel' does not exist")
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if onPremModelVer, exists := model["onpremModelVersion"]; exists {
				if onpremModelVerStr, ok := onPremModelVer.(string); ok {					
					updateModel.OnPremModelVer = onpremModelVerStr
					fmt.Printf("### onpremModelVerStr : [%s]\n", onpremModelVerStr)
				} else {
					msg := "'onpremModelVersion' is not a string type of value"
					log.Error().Msg(msg)

					newErr := fmt.Errorf("'onpremModelVersion' is not a string type of value")
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				msg := "'onpremModelVersion' does not exist"
				log.Error().Msg(msg)

				fmt.Println("'onpremModelVersion' does not exist")				
			}

		} 
		// else {
		// 	msg := "Error!! Error!! Error!! Error!! Error!!"
		// 	log.Error().Msg(msg)
		// }


		if model, ok := model.(map[string]interface{}); ok {
			if createTime, exists := model["createTime"]; exists {
				if createTimeStr, ok := createTime.(string); ok {
					// fmt.Printf("### createTimeStr : [%s]\n", createTimeStr)
					updateModel.CreateTime = createTimeStr
				} else {
					newErr := fmt.Errorf("'createTime' is not a string type of value")
					return newErr
					// return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				fmt.Println("'createTime' does not exist")

				newErr := fmt.Errorf("'createTime' does not exist")
				return newErr
			}
		}

		// onpremModelVer, err := getModuleVersion("github.com/cloud-barista/cm-model")
		// if err != nil {
		// 	fmt.Println("Error:", err)
		// } else {
		// 	fmt.Printf("On-premise Model version: %s\n", onpremModelVer)
		// }
		// updateModel.OnPremModelVer = onpremModelVer

		updateModel.Id = reqId
		updateModel.UpdateTime = getSeoulCurrentTime()

		updateModel.IsTargetModel = false
		updateModel.IsCloudModel = false

		// Convert to String type
		// strNum := strconv.Itoa(id)

		// Save the model to the key-value store
		lkvstore.Put(reqId, updateModel)

		// # Save the current state of the key-value store to file
		if err := lkvstore.SaveLkvStore(); err != nil {
			newErr := fmt.Errorf("Failed to Save the lkvstore to file. : [%v]", err)
			return c.JSON(http.StatusNotFound, newErr)
		} else {
			fmt.Println("Succeeded in Saving the lkvstore to file.")
		}
		
		// return c.JSON(http.StatusCreated, updateModel)
		// => Not http.StatusCreated

		// Get the model from the DB
		model, exists := lkvstore.Get(reqId)
		if exists {
			// fmt.Printf("Loaded value for [%s]: %v\n", c.Param("id"), model)	
			return c.JSON(http.StatusOK, model)
		} else {
			newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]", reqId)
			return c.JSON(http.StatusNotFound, newErr)
		}		
	} else {
		msg := "Failed to Find the Model from DB : [%s]\n"
		log.Error().Msg(msg)

		newErr := fmt.Errorf("[%s] : [%s]\n", msg, reqId)
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
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /onpremmodel/{id} [delete]
func DeleteOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### OnPrem Model ID to Delete : [%s]\n", c.Param("id"))

	// Verify loaded data without prefix
	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", c.Param("id"))

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						newErr := fmt.Errorf("The Given ID is Not a On-premise Model ID : [%s]", c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					} else {
						fmt.Println("This model is a Cloud Model!!")
					}
				} else {
					newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				newErr := fmt.Errorf("'isCloudModel' does not exist")
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		lkvstore.Delete(c.Param("id"))
	} else {
		newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		newErr := fmt.Errorf("Failed to Save the lkvstore to file. : [%v]", err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusOK, "Succeeded in Deleting the model")
}

// ##############################################################################################
// ### Cloud Migration User Model
// ##############################################################################################

type CloudModelReqInfo struct {
	UserId 					string    				`json:"userId"`
	IsTargetModel			bool	  				`json:"isTargetModel"`
	IsInitUserModel			bool	  				`json:"isInitUserModel"`
	UserModelName 			string  				`json:"userModelName"`
	Description 			string 					`json:"description"`
	UserModelVer			string  				`json:"userModelVersion"`
	CSP						string					`json:"csp"`
	Region					string					`json:"region"`
	Zone					string					`json:"zone"`
	CloudInfraModel			tbmodel.TbMciDynamicReq `json:"cloudInfraModel" validate:"required"`
}

type CloudModelRespInfo struct {
	Id   					string  				`json:"id"`
	UserId 					string    				`json:"userId"`
	IsTargetModel			bool	  				`json:"isTargetModel"`
	IsInitUserModel			bool	  				`json:"isInitUserModel"`
	UserModelName 			string  				`json:"userModelName"`
	Description 			string 					`json:"description"`
	UserModelVer			string  				`json:"userModelVersion"`
	CreateTime				string					`json:"createTime"`
	UpdateTime				string					`json:"updateTime"`
	CSP						string					`json:"csp"`
	Region					string					`json:"region"`
	Zone					string					`json:"zone"`
	IsCloudModel			bool					`json:"isCloudModel"`
	CloudModelVer			string 					`json:"cloudModelVersion"`
	CloudInfraModel			tbmodel.TbMciDynamicReq `json:"cloudInfraModel" validate:"required"`
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
			// fmt.Printf("\n# Model value : %v\n\n", model)
			if model, ok := model.(map[string]interface{}); ok {
				if isCloudModel, exists := model["isCloudModel"]; exists && isCloudModel == true {
					cloudModels = append(cloudModels, model)
				}
			}	
		}

		if len(cloudModels) < 1 {
			msg := "Failed to Find Any Model"
			log.Debug().Msg(msg)

			newErr := fmt.Errorf(msg)
			return c.JSON(http.StatusNotFound, newErr)
		}

		return c.JSON(http.StatusOK, cloudModels)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model from DB : [%s]", c.Param("id"))
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
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("# Model ID to Get : [%s]\n", c.Param("id"))

	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("\n# Loaded value for [%s]: %v\n\n", c.Param("id"), model)

		if model, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					if isCloudModelBool {
						fmt.Println("This model is a Cloud Model!!")						
					} else {
						newErr := fmt.Errorf("The Given ID is Not a Cloud Model ID : [%s]", c.Param("id"))
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
					newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				newErr := fmt.Errorf("'isCloudModel' does not exist")
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		return c.JSON(http.StatusOK, model)
	} else {
		newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]", c.Param("id"))
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
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### CreateCloudModelResp",)
	// spew.Dump(model)

    randomStr, err := generateRandomString(15)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Random 15-lenght of string:", randomStr)
    }
	model.Id = randomStr

	model.CreateTime = getSeoulCurrentTime()
	model.IsCloudModel = true

	cloudModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("Cloud Model version: %s\n", cloudModelVer)
	}
	model.CloudModelVer = cloudModelVer

	// Convert Int to String type
	// strNum := strconv.Itoa(randomNum)

	// Save the model to the key-value store
	lkvstore.Put(randomStr, model)

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		newErr := fmt.Errorf("Failed to Save the lkvstore to file. : [%v]", err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
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
// @Success 201 {object} UpdateCloudModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /cloudmodel/{id} [put]
func UpdateCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	reqId := c.Param("id")

	updateModel := new(UpdateCloudModelResp)

	if err := c.Bind(updateModel); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Printf("New Req Values for [%s]: %v\n\n", c.Param("id"), updateModel)

	model, exists := lkvstore.Get(reqId)
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", reqId)
		// fmt.Printf("# Cloud Model ID to Update : [%s]\n", reqId)
		// fmt.Printf("Values from DB [%s]: %v\n\n", c.Param("id"), model)	

		if cloudModel, ok := model.(map[string]interface{}); ok {
			// Check if the model is a on-premise model
			if isCloudModel, exists := cloudModel["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok {
					fmt.Printf("The value of isCloudModel is: %v\n", isCloudModel)

					if isCloudModelBool {
						fmt.Println("This model is a Cloud Model!!")						
					} else {
						newErr := fmt.Errorf("The Given ID is Not a Cloud Model ID : [%s]", reqId)
						return c.JSON(http.StatusNotFound, newErr)
					}
				} else {
					newErr := fmt.Errorf("'isCloudModel' is not a boolean type")
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				newErr := fmt.Errorf("'isCloudModel' does not exist")
				return c.JSON(http.StatusNotFound, newErr)
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if cloudModelVer, exists := model["cloudModelVersion"]; exists {
				if cloudModelVerStr, ok := cloudModelVer.(string); ok {
					updateModel.CloudModelVer = cloudModelVerStr
					fmt.Printf("### cloudModelVerStr : [%s]\n", cloudModelVerStr)
				} else {
					fmt.Println("'cloudModelVersion' is not a string type of value")
				}
			} else {
				fmt.Println("'cloudModelVersion' does not exist")
			}
		}

		if model, ok := model.(map[string]interface{}); ok {
			if createTime, exists := model["createTime"]; exists {
				if createTimeStr, ok := createTime.(string); ok {
					updateModel.CreateTime = createTimeStr
				} else {
					fmt.Println("'createTime' is not a string type of value")
				}
			} else {
				fmt.Println("'createTime' does not exist")
			}
		}
		
		// cloudModelVer, err := getModuleVersion("github.com/cloud-barista/cb-tumblebug")
		// if err != nil {
		// 	fmt.Println("Error:", err)
		// } else {
		// 	fmt.Printf("Cloud Model version: %s\n", cloudModelVer)
		// }
		// updateModel.CloudModelVer = cloudModelVer

		updateModel.Id = reqId
		updateModel.UpdateTime = getSeoulCurrentTime()
		updateModel.IsCloudModel = true

		// fmt.Println("### updateModel",)		
		// spew.Dump(updateModel)

		// Convert to String type
		// strNum := strconv.Itoa(id)

		// Save the model to the key-value store
		lkvstore.Put(reqId, updateModel)
		
		// # Save the current state of the key-value store to file
		if err := lkvstore.SaveLkvStore(); err != nil {
			newErr := fmt.Errorf("Failed to Save the lkvstore to file. : [%v]", err)
			return c.JSON(http.StatusNotFound, newErr)
		} else {
			fmt.Println("Succeeded in Saving the lkvstore to file.")
		}

		// Get the model from the DB
		model, exists := lkvstore.Get(reqId)
		if exists {
			// fmt.Printf("Loaded value for [%s]: %v\n", c.Param("id"), model)	
			return c.JSON(http.StatusOK, model)
		} else {
			newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]", reqId)
			return c.JSON(http.StatusNotFound, newErr)
		}
	} else {
		newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]", reqId)
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
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /cloudmodel/{id} [delete]
func DeleteCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### Model ID to Delete : [%s]\n", c.Param("id"))

	// Verify loaded data without prefix
	model, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", c.Param("id"))

		if model, ok := model.(map[string]interface{}); ok {
			if isCloudModel, exists := model["isCloudModel"]; exists {
				if isCloudModelBool, ok := isCloudModel.(bool); ok && isCloudModelBool {
					fmt.Println("This model is a Cloud Model!!")
				} else {
					newErr := fmt.Errorf("The Given ID is Not a Cloud Model ID : [%s]", c.Param("id"))
					return c.JSON(http.StatusNotFound, newErr)
				}
			} else {
				fmt.Println("'isCloudModel' does not exist")
			}
		}

		lkvstore.Delete(c.Param("id"))
	} else {
		newErr := fmt.Errorf("Failed to Find the Model from DB : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// # Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		newErr := fmt.Errorf("Failed to Save the lkvstore to file. : [%v]", err)
		return c.JSON(http.StatusNotFound, newErr)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusOK, "Succeeded in Deleting the model")
}
