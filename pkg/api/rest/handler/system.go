package handler

import (
	"net/http"

	"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-damselfly/pkg/common"
	"github.com/labstack/echo/v4"
)

type ResReadyz struct {
	model.SimpleMessage
}

// RestGetReadyz func check if CM-Damselfly server is ready or not.
// RestGetReadyz godoc
// @Summary Check Damselfly is ready
// @Description Check Damselfly is ready
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} ResReadyz
// @Failure 503 {object} ResReadyz
// @Router /readyz [get]
func RestGetReadyz(c echo.Context) error {
	message := ResReadyz{}
	message.Message = "CM-Damselfly is ready"
	if !common.SystemReady {
		message.Message = "CM-Damselfly is NOT ready"
		return c.JSON(http.StatusServiceUnavailable, &message)
	}
	return c.JSON(http.StatusOK, &message)
}

type ResHTTPVersion struct {
	model.SimpleMessage
}

// RestCheckHTTPVersion godoc
// @Summary Check HTTP version of incoming request
// @Description Checks and logs the HTTP version of the incoming request to the server console.
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} ResHTTPVersion
// @Failure 404 {object} ResHTTPVersion
// @Failure 500 {object} ResHTTPVersion
// @Router /httpVersion [get]
func RestCheckHTTPVersion(c echo.Context) error {
	// Access the *http.Request object from the echo.Context
	req := c.Request()

	// Determine the HTTP protocol version of the request
	okMessage := ResHTTPVersion{}
	okMessage.Message = req.Proto

	return c.JSON(http.StatusOK, &okMessage)
}
