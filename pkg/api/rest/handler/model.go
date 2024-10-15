package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"

	tbmodel "github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	onprem "github.com/cloud-barista/cm-model/infra/onprem"
)

// ##############################################################################################
// ### On-premise Infra Model
// ##############################################################################################

type OnPremInfraModelReqInfo struct {
	UserId 				string    			`json:"userid"`
	IsInitModel			bool	  			`json:"isinitmodel"`
	Name 				string  			`json:"name"`
	Description 		string 				`json:"description"`
	Version				string  			`json:"version"`
	OnPremInfra 		onprem.OnPremInfra 	`json:"onpreminfra" validate:"required"`
}

type OnPremInfraModelRespInfo struct {
	Id   				int    				`json:"id"`
	UserId 				string    			`json:"userid"`
	IsInitModel			bool	  			`json:"isinitmodel"`
	Name 				string  			`json:"name"`
	Description 		string 				`json:"description"`
	Version				string  			`json:"version"`
	CreateTime			string				`json:"createtime"`
	UpdateTime			string				`json:"updatetime"`
	IsCloudInfraModel	bool				`json:"iscloudinframodel"`
	OnPremInfra 		onprem.OnPremInfra 	`json:"onpreminfra" validate:"required"`
}
// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetOnPremInfraModelsResp struct {
	Models []OnPremInfraModelRespInfo `json:"models"`
}

// GetOnPremInfraModels godoc
// @Summary Get a list of on-premise infra models
// @Description Get a list of on-premise infra models.
// @Tags [API] On-Premise Infra Migration Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetOnPremInfraModelsResp "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
// @Router /onpreminfra [get]
func GetOnPremInfraModels(c echo.Context) error {

	// GetWithPrefix returns the values for a given key prefix.
	valueList, exists := lkvstore.GetWithPrefix("")
	if exists {
		// fmt.Printf("Loaded values : %v\n", valueList)
		return c.JSON(http.StatusOK, valueList)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type GetOnPremInfraModelResp struct {
	OnPremInfraModelRespInfo
}

// GetOnPremInfraModel godoc
// @Summary Get a specific on-premise infra model
// @Description Get a specific on-premise infra model.
// @Tags [API] On-Premise Infra Migration Models
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {object} GetOnPremInfraModelResp "(Sample) a model"
// @Failure 404 {object} object "model not found"
// @Router /onpreminfra/{id} [get]
func GetOnPremInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### OnPrem Model ID to Get : [%s]\n", c.Param("id"))

	// Verify loaded data without prefix
	value, exists := lkvstore.Get(c.Param("id"))
	if exists {
		// fmt.Printf("Loaded value for [%s]: %v\n", c.Param("id"), value)
		return c.JSON(http.StatusOK, value)
	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type CreateOnPremInfraModelReq struct {
	OnPremInfraModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type CreateOnPremInfraModelResp struct {
	OnPremInfraModelRespInfo
}

// CreateOnPremInfraModel godoc
// @Summary Create a new on-premise infra model
// @Description Create a new on-premise infra model with the given information.
// @Tags [API] On-Premise Infra Migration Models
// @Accept  json
// @Produce  json
// @Param Model body CreateOnPremInfraModelReq true "model information"
// @Success 201 {object} CreateOnPremInfraModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /onpreminfra [post]
func CreateOnPremInfraModel(c echo.Context) error {
	model := new(CreateOnPremInfraModelResp)
	if err := c.Bind(model); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### OnPremModel",)
	// spew.Dump(model)

	// # Incase of int type of ID
    randomNum, err := generateUnique15DigitInt()
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Random 15-digit number:", randomNum)
    }
	model.Id = randomNum
	model.CreateTime = getSeoulCurrentTime()
	model.IsCloudInfraModel = false

	// Convert Int to String type
	strNum := strconv.Itoa(randomNum)

	// Save the model to the key-value store
	lkvstore.Put(strNum, model)

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}	

	return c.JSON(http.StatusCreated, model)
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type UpdateOnPremInfraModelReq struct {
	OnPremInfraModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type UpdateOnPremInfraModelResp struct {
	OnPremInfraModelRespInfo
}

// UpdateOnPremInfraModel godoc
// @Summary Update a on-premise infra model
// @Description Update a on-premise infra model with the given information.
// @Tags [API] On-Premise Infra Migration Models
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Param Model body UpdateOnPremInfraModelReq true "Model information to update"
// @Success 201 {object} UpdateOnPremInfraModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /onpreminfra/{id} [put]
func UpdateOnPremInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	updateModel := new(UpdateOnPremInfraModelResp)
	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", c.Param("id"))
		fmt.Printf("### OnPrem Model ID to Update : [%s]\n", c.Param("id"))

		// updateModel = new(ReqUpdateOnPremModel)
		if err := c.Bind(updateModel); err != nil {
			return c.JSON(http.StatusBadRequest, "Invalid Request")
		}	
		updateModel.Id = id
		updateModel.UpdateTime = getSeoulCurrentTime()

		// fmt.Println("### MyOnPremModel",)		
		// spew.Dump(updateOnPremModel)

		// Convert to String type
		strNum := strconv.Itoa(id)

		// Save the model to the key-value store
		lkvstore.Put(strNum, updateModel)

	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}	

	return c.JSON(http.StatusCreated, updateModel)
}

// [Note]
// No RequestBody required for "DELETE /onpreminfra/{id}"

// [Note]
// No ResponseBody required for "DELETE /onpreminfra/{id}"

// DeleteOnPremInfraModel godoc
// @Summary Delete a on-premise infra model
// @Description Delete a on-premise infra model with the given information.
// @Tags [API] On-Premise Infra Migration Models
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /onpreminfra/{id} [delete]
func DeleteOnPremInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### OnPrem Model ID to Delete : [%s]\n", c.Param("id"))

	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", c.Param("id"))
		lkvstore.Delete(c.Param("id"))
	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusOK, "Succeeded in Deleting the model")
}

// ##############################################################################################
// ### Cloud Infra Model
// ##############################################################################################

type CloudInfraModelReqInfo struct {
	UserId 				string    				`json:"userid"`
	IsTargetModel		bool	  				`json:"istargetmodel"`
	IsInitModel			bool	  				`json:"isinitmodel"`
	Name 				string  				`json:"name"`
	Description 		string 					`json:"description"`
	Version				string  				`json:"version"`
	CSP					string					`json:"csp"`
	Region				string					`json:"region"`
	Zone				string					`json:"zone"`
	CloudInfra			tbmodel.TbMciDynamicReq `json:"cloudinfra" validate:"required"`
}

type CloudInfraModelRespInfo struct {
	Id   				int    					`json:"id"`
	UserId 				string    				`json:"userid"`
	IsTargetModel		bool	  				`json:"istargetmodel"`
	IsInitModel			bool	  				`json:"isinitmodel"`
	Name 				string  				`json:"name"`
	Description 		string 					`json:"description"`
	Version				string  				`json:"version"`
	CreateTime			string					`json:"createtime"`
	UpdateTime			string					`json:"updatetime"`
	CSP					string					`json:"csp"`
	Region				string					`json:"region"`
	Zone				string					`json:"zone"`
	IsCloudInfraModel	bool					`json:"iscloudinframodel"`
	CloudInfra			tbmodel.TbMciDynamicReq `json:"cloudinfra" validate:"required"`
}
// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type GetCloudInfraModelsResp struct {
	Models []CloudInfraModelRespInfo `json:"models"`
}

// GetCloudInfraModels godoc
// @Summary Get a list of cloud infra models
// @Description Get a list of cloud infra models.
// @Tags [API] Cloud Infra Migration Models
// @Accept  json
// @Produce  json
// @Success 200 {object} GetCloudInfraModelsResp "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
// @Router /cloudinfra [get]
func GetCloudInfraModels(c echo.Context) error {

	// GetWithPrefix returns the values for a given key prefix.
	valueList, exists := lkvstore.GetWithPrefix("")
	if exists {
		// fmt.Printf("Loaded values : %v\n", valueList)
		return c.JSON(http.StatusOK, valueList)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type GetCloudInfraModelResp struct {
	CloudInfraModelRespInfo
}

// GetCloudInfraModel godoc
// @Summary Get a specific cloud infra model
// @Description Get a specific cloud infra model.
// @Tags [API] Cloud Infra Migration Models
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {object} GetCloudInfraModelResp "(Sample) a model"
// @Failure 404 {object} object "model not found"
// @Router /cloudinfra/{id} [get]
func GetCloudInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### Cloud Model ID to Get : [%s]\n", c.Param("id"))

	// Verify loaded data without prefix
	value, exists := lkvstore.Get(c.Param("id"))
	if exists {
		// fmt.Printf("Loaded value for [%s]: %v\n", c.Param("id"), value)
		return c.JSON(http.StatusOK, value)
	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type CreateCloudInfraModelReq struct {
	CloudInfraModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type CreateCloudInfraModelResp struct {
	CloudInfraModelRespInfo
}

// CreateCloudInfraModel godoc
// @Summary Create a new cloud infra model
// @Description Create a new cloud infra model with the given information.
// @Tags [API] Cloud Infra Migration Models
// @Accept  json
// @Produce  json
// @Param Model body CreateCloudInfraModelReq true "model information"
// @Success 201 {object} CreateCloudInfraModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /cloudinfra [post]
func CreateCloudInfraModel(c echo.Context) error {
	model := new(CreateCloudInfraModelResp)
	if err := c.Bind(model); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### CreateCloudInfraModelResp",)
	// spew.Dump(model)

	// # Incase of int type of ID
    randomNum, err := generateUnique15DigitInt()
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Random 15-digit number:", randomNum)
    }
	model.Id = randomNum
	model.CreateTime = getSeoulCurrentTime()
	model.IsCloudInfraModel = true

	// Convert Int to String type
	strNum := strconv.Itoa(randomNum)

	// Save the model to the key-value store
	lkvstore.Put(strNum, model)

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}	

	return c.JSON(http.StatusCreated, model)
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type UpdateCloudInfraModelReq struct {
	CloudInfraModelReqInfo
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type UpdateCloudInfraModelResp struct {
	CloudInfraModelRespInfo
}

// UpdateCloudInfraModel godoc
// @Summary Update a cloud infra model
// @Description Update a cloud infra model with the given information.
// @Tags [API] Cloud Infra Migration Models
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Param Model body UpdateCloudInfraModelReq true "Model information to update"
// @Success 201 {object} UpdateCloudInfraModelResp "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /cloudinfra/{id} [put]
func UpdateCloudInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	updateModel := new(UpdateCloudInfraModelResp)
	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", c.Param("id"))
		fmt.Printf("### Cloud Model ID to Update : [%s]\n", c.Param("id"))

		if err := c.Bind(updateModel); err != nil {
			return c.JSON(http.StatusBadRequest, "Invalid Request")
		}	
		updateModel.Id = id
		updateModel.UpdateTime = getSeoulCurrentTime()

		// fmt.Println("### updateModel",)		
		// spew.Dump(updateModel)

		// Convert to String type
		strNum := strconv.Itoa(id)

		// Save the model to the key-value store
		lkvstore.Put(strNum, updateModel)

	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}	

	return c.JSON(http.StatusCreated, updateModel)
}

// [Note]
// No RequestBody required for "DELETE /cloudinfra/{id}"

// [Note]
// No ResponseBody required for "DELETE /cloudinfra/{id}"

// DeleteCloudInfraModel godoc
// @Summary Delete a cloud infra model
// @Description Delete a cloud infra model with the given information.
// @Tags [API] Cloud Infra Migration Models
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /cloudinfra/{id} [delete]
func DeleteCloudInfraModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### Model ID to Delete : [%s]\n", c.Param("id"))

	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : [%s]\n", c.Param("id"))
		lkvstore.Delete(c.Param("id"))
	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}

	// Save the current state of the key-value store to file
	if err := lkvstore.SaveLkvStore(); err != nil {
		fmt.Printf("Failed to Save the lkvstore to file. : [%v]\n", err)
	} else {
		fmt.Println("Succeeded in Saving the lkvstore to file.")
	}

	return c.JSON(http.StatusOK, "Succeeded in Deleting the model")
}
