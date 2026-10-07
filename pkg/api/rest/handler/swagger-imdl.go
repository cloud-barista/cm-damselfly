package handler

import (
	"bytes"
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
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/swaggo/swag"
)

// [Note]
// Swagger UI loads an API definition once, so the body schema cannot follow the value typed into the
// 'onpremModelVersion' / 'cloudModelVersion' query parameters. Instead, an API definition per cm-beetle/imdl
// tagged version is provided and listed in the 'Select a definition' drop-down of Swagger UI. In the definition
// of a version, the body of CreateInfraModel / UpdateInfraModel follows the 'OnpremInfra' / 'RecommendedInfra'
// struct of that version, and the version query parameters default to that version.

const (
	defaultSwaggerDocURL      = "doc.json"
	defaultSwaggerDocYAMLURL  = "doc.yaml"
	imdlSwaggerDocURLTemplate = "imdl/%s/doc.json"
)

var (
	imdlSwaggerDocCacheMu sync.Mutex
	imdlSwaggerDocCache   = map[string][]byte{}
)

// SwaggerUIHandler serves Swagger UI, listing the default API definition and one API definition per
// cm-beetle/imdl and cm-grasshopper/smdl tagged version (latest first) in the 'Select a definition' drop-down.
func SwaggerUIHandler(c echo.Context) error {
	urls := []string{defaultSwaggerDocURL, defaultSwaggerDocYAMLURL}
	if strings.HasSuffix(c.Request().URL.Path, "/index.html") {
		// Swagger UI keeps the selected definition in the query string (e.g. '?urls.primaryName=...'),
		// which echo-swagger would otherwise take as a part of the file name.
		c.Request().RequestURI = c.Request().URL.Path

		versions, err := getInfraModelVersions()
		if err != nil {
			log.Warn().Msgf("Failed to get the cm-beetle/imdl versions for Swagger UI : [%v]", err)
		}
		for i := len(versions) - 1; i >= 0; i-- {
			urls = append(urls, fmt.Sprintf(imdlSwaggerDocURLTemplate, versions[i]))
		}

		swVersions, err := getSoftwareModelVersions()
		if err != nil {
			log.Warn().Msgf("Failed to get the cm-grasshopper/smdl versions for Swagger UI : [%v]", err)
		}
		for i := len(swVersions) - 1; i >= 0; i-- {
			urls = append(urls, fmt.Sprintf(smdlSwaggerDocURLTemplate, swVersions[i]))
		}
	}
	return echoSwagger.EchoWrapHandler(func(config *echoSwagger.Config) {
		config.URLs = urls
	})(c)
}

// GetImdlVersionSwaggerDoc serves the API definition (Swagger 2.0) of the given cm-beetle/imdl tagged version.
func GetImdlVersionSwaggerDoc(c echo.Context) error {
	version := strings.TrimSpace(c.Param("version"))

	versions, err := getInfraModelVersions()
	if err != nil {
		newErr := fmt.Errorf("failed to get the tagged cm-beetle/imdl model versions: %w", err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}
	if _, err := selectModelVersion(version, versions); version == "" || err != nil {
		newErr := fmt.Errorf("'%s' is not a cm-beetle/imdl tagged version", version)
		log.Warn().Msg(newErr.Error())
		return c.JSON(http.StatusNotFound, model.Response{Success: false, Text: newErr.Error()})
	}

	doc, err := getImdlVersionSwaggerDoc(c.Request().Context(), version)
	if err != nil {
		newErr := fmt.Errorf("failed to generate the API definition of cm-beetle/imdl %s : [%v]", version, err)
		log.Error().Msg(newErr.Error())
		return c.JSON(http.StatusInternalServerError, model.Response{Success: false, Text: newErr.Error()})
	}
	return c.JSONBlob(http.StatusOK, doc)
}

func getImdlVersionSwaggerDoc(ctx context.Context, version string) ([]byte, error) {
	imdlSwaggerDocCacheMu.Lock()
	cached, ok := imdlSwaggerDocCache[version]
	imdlSwaggerDocCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	doc, err := buildImdlVersionSwaggerDoc(ctx, version)
	if err != nil {
		return nil, err
	}

	imdlSwaggerDocCacheMu.Lock()
	imdlSwaggerDocCache[version] = doc
	imdlSwaggerDocCacheMu.Unlock()
	return doc, nil
}

// buildImdlVersionSwaggerDoc derives the API definition of a cm-beetle/imdl version from the default one.
// Only CreateInfraModel / UpdateInfraModel are changed; the other APIs keep the built-in struct.
func buildImdlVersionSwaggerDoc(ctx context.Context, version string) ([]byte, error) {
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
	baseReqDef, ok := defs["handler.CreateInfraModelReq"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("the API definition has no 'handler.CreateInfraModelReq'")
	}

	defPrefix := "imdl-" + version + "."
	modelSchemas := map[string]interface{}{}
	for _, schema := range []infraModelSchemaInfo{onpremInfraModelSchema, cloudInfraModelSchema} {
		pkg, err := modelschema.LoadPackage(ctx, infraModelSchemaSource, version, schema.PkgDir)
		if err != nil {
			return nil, err
		}
		if !pkg.HasType(schema.RootType) {
			modelSchemas[schema.FieldName] = map[string]interface{}{
				"type":        "object",
				"description": fmt.Sprintf("'%s' struct does not exist in cm-beetle/imdl %s", schema.RootType, version),
			}
			continue
		}
		ref, err := pkg.AddSwaggerDefinitions(defs, schema.RootType, defPrefix+strings.ReplaceAll(schema.PkgDir, "-", "")+".")
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
	reqDefName := defPrefix + "handler.CreateInfraModelReq"
	defs[reqDefName] = reqDef

	paths, _ := spec["paths"].(map[string]interface{})
	for _, op := range []struct{ path, method string }{{"/infra-model", "post"}, {"/infra-model/{id}", "put"}} {
		pathItem, _ := paths[op.path].(map[string]interface{})
		operation, _ := pathItem[op.method].(map[string]interface{})
		params, _ := operation["parameters"].([]interface{})
		for _, p := range params {
			param, _ := p.(map[string]interface{})
			switch {
			case param["in"] == "body":
				param["schema"] = map[string]interface{}{"$ref": "#/definitions/" + reqDefName}
			case param["in"] == "query" && (param["name"] == "onpremModelVersion" || param["name"] == "cloudModelVersion"):
				param["default"] = version
			}
		}
		if desc, ok := operation["description"].(string); ok {
			operation["description"] = desc + fmt.Sprintf("\n\n(This API definition shows the request body of cm-beetle/imdl %s.)", version)
		}
	}

	if info, ok := spec["info"].(map[string]interface{}); ok {
		info["title"] = fmt.Sprintf("%v (cm-beetle/imdl %s)", info["title"], version)
		info["description"] = fmt.Sprintf("%v\n\nThe request body of 'POST /infra-model' and 'PUT /infra-model/{id}' follows the 'OnpremInfra' / 'RecommendedInfra' struct of cm-beetle/imdl %s.", info["description"], version)
	}

	return marshalInTopLevelKeyOrder(spec, baseDoc)
}

// marshalInTopLevelKeyOrder marshals the spec keeping the top-level key order of the original document.
// Swagger UI (swagger-client) does not merge 'allOf' when generating example values if 'definitions' precedes 'paths',
// which happens with the alphabetical key order of encoding/json.
func marshalInTopLevelKeyOrder(spec map[string]interface{}, originalDoc string) ([]byte, error) {
	var order []string
	dec := json.NewDecoder(strings.NewReader(originalDoc))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return nil, err
		}
		order = append(order, keyToken.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	written := map[string]bool{}
	writeKey := func(key string) error {
		value, ok := spec[key]
		if !ok || written[key] {
			return nil
		}
		written[key] = true
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(key)
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return err
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.Write(valueJSON)
		return nil
	}
	for _, key := range order {
		if err := writeKey(key); err != nil {
			return nil, err
		}
	}
	for key := range spec {
		if err := writeKey(key); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func deepCopyJSON(v interface{}) interface{} {
	switch vv := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(vv))
		for k, val := range vv {
			out[k] = deepCopyJSON(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(vv))
		for i, val := range vv {
			out[i] = deepCopyJSON(val)
		}
		return out
	}
	return v
}
