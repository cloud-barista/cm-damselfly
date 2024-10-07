package handler

import (	
	"fmt"
	"net/http"
	"strconv"
	"strings"
	// "github.com/davecgh/go-spew/spew"
	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	onprem "github.com/cloud-barista/cm-model/infra/onprem"
)

type MyModel struct {
	Id   		int    					`json:"id"`
	Name 		string  				`json:"name"`
	Description string 					`json:"description"`
	Version		string  				`json:"version"`
	Network 	onprem.NetworkProperty  `json:"network,omitempty"`
	Servers 	[]onprem.ServerProperty `json:"servers" validate:"required"`
	// TODO: Add other fields
}
// Caution!!)
// Init Swagger : ]# swag init --parseDependency --parseInternal
// Need to add '--parseDependency --parseInternal' in order to apply imported structures

type ResGetModels struct {
	Models []MyModel `json:"models"`
}

// GetModels godoc
// @Summary Get a list of models
// @Description Get a list of models.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Success 200 {object} ResGetModels "(sample) This is a list of models"
// @Failure 404 {object} object "model not found"
// @Router /model [get]
func GetModels(c echo.Context) error {

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

type ResGetModel struct {
	MyModel
}
// type ResGetModel struct {
// 	onprem.OnPremInfra
// }

// GetModel godoc
// @Summary Get a specific model
// @Description Get a specific model.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {object} ResGetModel "(Sample) a model"
// @Failure 404 {object} object "model not found"
// @Router /model/{id} [get]
func GetModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}
	fmt.Printf("### MyModel ID to Get : [%s]", c.Param("id"))

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
// Struct Embedding is used to inherit the fields of MyModel
type ReqCreateModel struct {
	MyModel
}

// CreateModel godoc
// @Summary Create a new model
// @Description Create a new model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param Model body ReqCreateModel true "model information"
// @Success 201 {object} ResCreateModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /model [post]
func CreateModel(c echo.Context) error {
	model := new(ReqCreateModel)
	if err := c.Bind(model); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}
	// fmt.Println("### MyModel",)
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
// Struct Embedding is used to inherit the fields of MyModel
type ReqUpdateModel struct {
	MyModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyModel
type ResUpdateModel struct {
	MyModel
}

// UpdateModel godoc
// @Summary Update a model
// @Description Update a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Param Model body ReqUpdateModel true "Model information to update"
// @Success 201 {object} ResUpdateModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Router /model/{id} [put]
func UpdateModel(c echo.Context) error {
	if strings.EqualFold(c.Param("id"), "") {
		return c.JSON(http.StatusBadRequest, "Invalid ID!!")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	updateModel := new(ReqUpdateModel)
	// Verify loaded data without prefix
	_, exists := lkvstore.Get(c.Param("id"))
	if exists {
		fmt.Printf("Succeeded in Finding the model : '%s'\n", c.Param("id"))
		fmt.Printf("### MyModel ID to Update : [%s]", c.Param("id"))

		// updateModel = new(ReqUpdateModel)
		if err := c.Bind(updateModel); err != nil {
			return c.JSON(http.StatusBadRequest, "Invalid Request")
		}	
		updateModel.Id = id
		// fmt.Println("### MyModel",)		
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
// Struct Embedding is used to inherit the fields of MyModel
type ReqPatchModel struct {
	MyModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyModel
type ResPatchModel struct {
	MyModel
}

// PatchModel godoc
// @Summary Patch a model
// @Description Patch a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Param Model body ReqPatchModel true "Model information to update"
// @Success 200 {object} ResPatchModel "(Sample) This is a sample description for success response in Swagger UI"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /model/{id} [patch]
func PatchModel(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	u := new(ReqPatchModel)
	if err := c.Bind(u); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}
	u.Id = id

	return c.JSON(http.StatusOK, u)
}

// [Note]
// No RequestBody required for "DELETE /model/{id}"

// [Note]
// No ResponseBody required for "DELETE /model/{id}"

// DeleteModel godoc
// @Summary Delete a model
// @Description Delete a model with the given information.
// @Tags [API] Cloud Migration Models (TBD)
// @Accept  json
// @Produce  json
// @Param id path int true "Model ID"
// @Success 200 {string} string "Model deletion successful"
// @Failure 400 {object} object "Invalid Request"
// @Failure 404 {object} object "Model Not Found"
// @Router /model/{id} [delete]
func DeleteModel(c echo.Context) error {
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
