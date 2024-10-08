package handler

import (	
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"github.com/labstack/echo/v4"
	// "github.com/davecgh/go-spew/spew"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	onprem "github.com/cloud-barista/cm-model/infra/onprem"
	tbmodel "github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// ##############################################################################################
// ### On-premise Infra Model
// ##############################################################################################

type MyOnPremModel struct {
	Id   		int    					`json:"id"`
	Name 		string  				`json:"name"`
	Description string 					`json:"description"`
	Version		string  				`json:"version"`
	OnPremInfra onprem.OnPremInfra 		`json:"onpreminfra"`	
	// TODO: Add other fields
}
// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type ResGetOnPremModels struct {
	Models []MyOnPremModel `json:"models"`
}

// GetOnPremModels godoc
// @Summary Get a list of models
// @Description Get a list of models.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Success 200 {object} ResGetOnPremModels "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
// @Router /model/onprem [get]
func GetOnPremModels(c echo.Context) error {

	// GetWithPrefix returns the values for a given key prefix.
	valueList, exists := lkvstore.GetWithPrefix("")
	if exists {
		fmt.Printf("Loaded values : %v\n", valueList)
		return c.JSON(http.StatusOK, valueList)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type ResGetOnPremModel struct {
	MyOnPremModel
}

// GetOnPremModel godoc
// @Summary Get a specific model
// @Description Get a specific model.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {object} ResGetOnPremModel "(Sample) a model"
// @Failure 404 {object} object "model not found"
// @Router /model/onprem/{id} [get]
func GetOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### MyOnPremModel ID to Get : [%s]", c.Param("id"))

	// Verify loaded data without prefix
	value, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Loaded value for '%s': %v\n", c.Param("id"), value)
		return c.JSON(http.StatusOK, value)
	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type ReqCreateOnPremModel struct {
	MyOnPremModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type ResCreateOnPremModel struct {
	MyOnPremModel
}

// CreateOnPremModel godoc
// @Summary Create a new model
// @Description Create a new model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param Model body ReqCreateOnPremModel true "model information"
// @Success 201 {object} ResCreateOnPremModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /model/onprem [post]
func CreateOnPremModel(c echo.Context) error {
	model := new(ReqCreateOnPremModel)
	if err := c.Bind(model); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### MyOnPremModel",)
	// spew.Dump(model)

	// # Incase of int type of ID
    randomNum, err := generateUnique15DigitInt()
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Random 15-digit number:", randomNum)
    }
	model.Id = randomNum

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
type ReqUpdateOnPremModel struct {
	MyOnPremModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyOnPremModel
type ResUpdateOnPremModel struct {
	MyOnPremModel
}

// UpdateOnPremModel godoc
// @Summary Update a model
// @Description Update a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Param Model body ReqUpdateOnPremModel true "Model information to update"
// @Success 201 {object} ResUpdateOnPremModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /model/onprem/{id} [put]
func UpdateOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	updateModel := new(ReqUpdateOnPremModel)
	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : '%s'\n", c.Param("id"))
		fmt.Printf("### MyOnPremModel ID to Update : [%s]", c.Param("id"))

		// updateModel = new(ReqUpdateOnPremModel)
		if err := c.Bind(updateModel); err != nil {
			return c.JSON(http.StatusBadRequest, "Invalid Request")
		}	
		updateModel.Id = id
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
// No RequestBody required for "DELETE /model/{id}"

// [Note]
// No ResponseBody required for "DELETE /model/{id}"

// DeleteOnPremModel godoc
// @Summary Delete a model
// @Description Delete a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /model/onprem/{id} [delete]
func DeleteOnPremModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### Model ID to Delete : [%s]", c.Param("id"))

	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : '%s'\n", c.Param("id"))
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

type MyCloudModel struct {
	Id   			int    					`json:"id"`
	IsTargetModel	bool	  				`json:"istargetmodel"`	
	// Name 		string  				`json:"name"`
	// Description 	string 					`json:"description"`
	Version			string  				`json:"version"`
	CloudInfra		tbmodel.TbMciDynamicReq `json:"cloudinfra" validate:"required"`
	// TODO: Add other fields
}
// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type ResGetCloudModels struct {
	Models []MyCloudModel `json:"models"`
}

// GetCloudModels godoc
// @Summary Get a list of models
// @Description Get a list of models.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Success 200 {object} ResGetCloudModels "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
// @Router /model/cloud [get]
func GetCloudModels(c echo.Context) error {

	// GetWithPrefix returns the values for a given key prefix.
	valueList, exists := lkvstore.GetWithPrefix("")
	if exists {
		fmt.Printf("Loaded values : %v\n", valueList)
		return c.JSON(http.StatusOK, valueList)
	} else {
		newErr := fmt.Errorf("Failed to Find Any Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

type ResGetCloudModel struct {
	MyCloudModel
}

// GetCloudModel godoc
// @Summary Get a specific model
// @Description Get a specific model.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {object} ResGetCloudModel "(Sample) a model"
// @Failure 404 {object} object "model not found"
// @Router /model/cloud/{id} [get]
func GetCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### MyCloudModel ID to Get : [%s]", c.Param("id"))

	// Verify loaded data without prefix
	value, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Loaded value for '%s': %v\n", c.Param("id"), value)
		return c.JSON(http.StatusOK, value)
	} else {
		newErr := fmt.Errorf("Failed to Find the Model : [%s]\n", c.Param("id"))
		return c.JSON(http.StatusNotFound, newErr)
	}
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type ReqCreateCloudModel struct {
	MyCloudModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type ResCreateCloudModel struct {
	MyCloudModel
}

// CreateCloudModel godoc
// @Summary Create a new model
// @Description Create a new model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param Model body ReqCreateCloudModel true "model information"
// @Success 201 {object} ResCreateCloudModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /model/cloud [post]
func CreateCloudModel(c echo.Context) error {
	model := new(ReqCreateCloudModel)
	if err := c.Bind(model); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### MyCloudModel",)
	// spew.Dump(model)

	// # Incase of int type of ID
    randomNum, err := generateUnique15DigitInt()
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Random 15-digit number:", randomNum)
    }
	model.Id = randomNum

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
type ReqUpdateCloudModel struct {
	MyCloudModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyCloudModel
type ResUpdateCloudModel struct {
	MyCloudModel
}

// UpdateCloudModel godoc
// @Summary Update a model
// @Description Update a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Param Model body ReqUpdateCloudModel true "Model information to update"
// @Success 201 {object} ResUpdateCloudModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /model/cloud/{id} [put]
func UpdateCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	updateModel := new(ReqUpdateCloudModel)
	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : '%s'\n", c.Param("id"))
		fmt.Printf("### MyCloudModel ID to Update : [%s]", c.Param("id"))

		// updateModel = new(ReqUpdateCloudModel)
		if err := c.Bind(updateModel); err != nil {
			return c.JSON(http.StatusBadRequest, "Invalid Request")
		}	
		updateModel.Id = id
		// fmt.Println("### MyCloudModel",)		
		// spew.Dump(updateCloudModel)

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
// No RequestBody required for "DELETE /model/{id}"

// [Note]
// No ResponseBody required for "DELETE /model/{id}"

// DeleteCloudModel godoc
// @Summary Delete a model
// @Description Delete a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /model/cloud/{id} [delete]
func DeleteCloudModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### Model ID to Delete : [%s]", c.Param("id"))

	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : '%s'\n", c.Param("id"))
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
