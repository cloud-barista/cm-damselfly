package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/cloud-barista/cm-damselfly/pkg/api/rest/model"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	echo "github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/swaggo/swag"
)

// [Note]
// Like the cm-beetle/imdl API definitions (see swagger-imdl.go), an API definition per cm-grasshopper/smdl
// tagged version is provided and listed in the 'Select a definition' drop-down of Swagger UI. In the definition
// of a version, the body of CreateSoftwareModel / UpdateSoftwareModel follows the 'SourceGroupSoftwareProperty' /
// 'TargetGroupSoftwareProperty' struct of that version, and the 'softwareModelVersion' query parameter defaults to that version.

const smdlSwaggerDocURLTemplate = "smdl/%s/doc.json"

var (
	smdlSwaggerDocCacheMu sync.Mutex
	smdlSwaggerDocCache   = map[string][]byte{}
)

// GetSmdlVersionSwaggerDoc serves the API definition (Swagger 2.0) of the given cm-grasshopper/smdl tagged version.
func GetSmdlVersionSwaggerDoc(c echo.Context) error {
	version := strings.TrimSpace(c.Param("version"))

	versions, err := getSoftwareModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-grasshopper/smdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}
	if _, err := selectModelVersion(version, versions); version == "" || err != nil {
		newErr := fmt.Errorf("'%s' is not a cm-grasshopper/smdl tagged version", version)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusNotFound, model.Response{Success: false, Text: newErr.Error()})
	}

	doc, err := getSmdlVersionSwaggerDoc(c.Request().Context(), version)
	if err != nil {
		newErr := fmt.Errorf("failed to generate the API definition of cm-grasshopper/smdl %s : [%v]", version, err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}
	return c.JSONBlob(http.StatusOK, doc)
}

func getSmdlVersionSwaggerDoc(ctx context.Context, version string) ([]byte, error) {
	smdlSwaggerDocCacheMu.Lock()
	cached, ok := smdlSwaggerDocCache[version]
	smdlSwaggerDocCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	doc, err := buildSmdlVersionSwaggerDoc(ctx, version)
	if err != nil {
		return nil, err
	}

	smdlSwaggerDocCacheMu.Lock()
	smdlSwaggerDocCache[version] = doc
	smdlSwaggerDocCacheMu.Unlock()
	return doc, nil
}

// buildSmdlVersionSwaggerDoc derives the API definition of a cm-grasshopper/smdl version from the default one.
// Only CreateSoftwareModel / UpdateSoftwareModel are changed; the other APIs keep the built-in struct.
func buildSmdlVersionSwaggerDoc(ctx context.Context, version string) ([]byte, error) {
	baseDoc, err := swag.ReadDoc()
	if err != nil {
		return nil, err
	}
	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(baseDoc), &spec); err != nil {
		return nil, err
	}

	defs, ok := spec["definitions"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("the API definition has no 'definitions'")
	}
	baseReqDef, ok := defs["handler.CreateSoftwareModelReq"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("the API definition has no 'handler.CreateSoftwareModelReq'")
	}

	pkg, err := modelschema.LoadPackage(ctx, softwareModelSchemaSource, version, "")
	if err != nil {
		return nil, err
	}

	defPrefix := "smdl-" + version + "."
	modelSchemas := map[string]interface{}{}
	for _, schema := range []softwareModelSchemaInfo{sourceSoftwareModelSchema, targetSoftwareModelSchema} {
		if !pkg.HasType(schema.RootType) {
			modelSchemas[schema.FieldName] = map[string]interface{}{
				"type":        "object",
				"description": fmt.Sprintf("'%s' struct does not exist in cm-grasshopper/smdl %s", schema.RootType, version),
			}
			continue
		}
		ref, err := pkg.AddSwaggerDefinitions(defs, schema.RootType, defPrefix+"softwaremodel.")
		if err != nil {
			return nil, err
		}
		modelSchemas[schema.FieldName] = ref
	}

	reqDef := deepCopyJSON(baseReqDef).(map[string]interface{})
	props, _ := reqDef["properties"].(map[string]interface{})
	if props == nil {
		props = map[string]interface{}{}
		reqDef["properties"] = props
	}
	for fieldName, schema := range modelSchemas {
		props[fieldName] = schema
	}
	reqDefName := defPrefix + "handler.CreateSoftwareModelReq"
	defs[reqDefName] = reqDef

	paths, _ := spec["paths"].(map[string]interface{})
	for _, op := range []struct{ path, method string }{{"/software-model", "post"}, {"/software-model/{id}", "put"}} {
		pathItem, _ := paths[op.path].(map[string]interface{})
		operation, _ := pathItem[op.method].(map[string]interface{})
		params, _ := operation["parameters"].([]interface{})
		for _, p := range params {
			param, _ := p.(map[string]interface{})
			switch {
			case param["in"] == "body":
				param["schema"] = map[string]interface{}{"$ref": "#/definitions/" + reqDefName}
			case param["in"] == "query" && param["name"] == "softwareModelVersion":
				param["default"] = version
			}
		}
		if desc, ok := operation["description"].(string); ok {
			operation["description"] = desc + fmt.Sprintf("\n\n(This API definition shows the request body of cm-grasshopper/smdl %s.)", version)
		}
	}

	if info, ok := spec["info"].(map[string]interface{}); ok {
		info["title"] = fmt.Sprintf("%v (cm-grasshopper/smdl %s)", info["title"], version)
		info["description"] = fmt.Sprintf("%v\n\nThe request body of 'POST /software-model' and 'PUT /software-model/{id}' follows the 'SourceGroupSoftwareProperty' / 'TargetGroupSoftwareProperty' struct of cm-grasshopper/smdl %s.", info["description"], version)
	}

	return marshalInTopLevelKeyOrder(spec, baseDoc)
}
