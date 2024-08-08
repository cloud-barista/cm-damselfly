package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type MyModel struct {
	Name string `json:"name"`
	Id   int    `json:"id"`
}

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

	// In this example, hardcoded data is returned
	models := []MyModel{
		{Id: 1, Name: "AAA"},
		{Id: 2, Name: "BBB"},
		{Id: 3, Name: "CCC"},
	}
	return c.JSON(http.StatusOK, models)
}

type ResGetModel struct {
	MyModel
}

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
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	// Implement model retrieval logic (this is a simple example)
	if id != 1 {
		return c.JSON(http.StatusNotFound, "model not found")
	}

	// In this example, hardcoded data is returned
	model := MyModel{Id: 1, Name: "AAA"}
	return c.JSON(http.StatusOK, model)
}

// [Note]
// Struct Embedding is used to inherit the fields of MyModel
type ReqCreateModel struct {
	MyModel
}

// [Note]
// Struct Embedding is used to inherit the fields of MyModel
type ResCreateModel struct {
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
	u := new(ReqCreateModel)
	if err := c.Bind(u); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid Request")
	}

	// Implement model creation logic (this is a simple example)
	u.Id = 100 // Unique ID generation logic needed in actual implementation

	return c.JSON(http.StatusCreated, u)
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
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	u := new(ReqUpdateModel)
	if err := c.Bind(u); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}

	// Implement model update logic (this is a simple example)
	u.Id = id // Update the information of the model with the corresponding ID in the actual implementation

	return c.JSON(http.StatusOK, u)
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

	// Implement model update logic (this is a simple example)
	u.Id = id // Update the information of the model with the corresponding ID in the actual implementation

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
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid ID format")
	}

	// Implement model update logic (this is a simple example)
	// In this example, hardcoded data is returned
	model := MyModel{Id: 1, Name: "AAA"}
	if id != model.Id {
		return c.JSON(http.StatusNotFound, "Model not found")
	}

	return c.JSON(http.StatusOK, "Model deletion successful")
}
