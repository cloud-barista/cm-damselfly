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
// 'modelVersion' query parameter. Instead, an API definition per cm-beetle/imdl tagged version is provided and
// listed in the 'Select a definition' drop-down of Swagger UI. In the definition of a version, the body of
// CreateInfraModel / UpdateInfraModel follows the 'OnpremInfra' / 'RecommendedInfra' struct of that version,
// and the 'modelVersion' query parameter defaults to that version. Likewise, the body of CreateRDBMSModel / UpdateRDBMSModel
// follows the 'SourceRDBMS' / 'RecommendedRDBMS' struct of that version (v0.1.13 or later), and 'rdbmsModelVersion' defaults to it.
// The body of CreateObjectStorageModel / UpdateObjectStorageModel follows the 'SourceObjectStorage' / 'RecommendedObjectStorage' struct
// of that version (v0.1.3 or later), and 'objectStorageModelVersion' defaults to it.

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
// Only CreateInfraModel / UpdateInfraModel / CreateRDBMSModel / UpdateRDBMSModel / CreateObjectStorageModel / UpdateObjectStorageModel (the body)
// and the version parameters of CreateInfraModel / ConvertInfraModel / CreateRDBMSModel / ConvertRDBMSModel / CreateObjectStorageModel /
// ConvertObjectStorageModel (the default) are changed; the other APIs keep the built-in struct.
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
			case param["in"] == "query" && param["name"] == "modelVersion":
				param["default"] = version
			}
		}
		if desc, ok := operation["description"].(string); ok {
			operation["description"] = desc + fmt.Sprintf("\n\n(This API definition shows the request body of cm-beetle/imdl %s.)", version)
		}
	}

	convertPathItem, _ := paths["/infra-model/{id}/convert"].(map[string]interface{})
	convertOperation, _ := convertPathItem["get"].(map[string]interface{})
	convertParams, _ := convertOperation["parameters"].([]interface{})
	for _, p := range convertParams {
		param, _ := p.(map[string]interface{})
		if param["in"] == "query" && param["name"] == "modelVersion" {
			param["default"] = version
		}
	}

	if err := addRDBMSModelToImdlSwaggerDoc(ctx, defs, paths, version); err != nil {
		return nil, err
	}
	if err := addObjectStorageModelToImdlSwaggerDoc(ctx, defs, paths, version); err != nil {
		return nil, err
	}

	if info, ok := spec["info"].(map[string]interface{}); ok {
		info["title"] = fmt.Sprintf("%v (cm-beetle/imdl %s)", info["title"], version)
		info["description"] = fmt.Sprintf("%v\n\nThe request body of 'POST /infra-model' and 'PUT /infra-model/{id}' follows the 'OnpremInfra' / 'RecommendedInfra' struct of cm-beetle/imdl %s.", info["description"], version)
		if compareModuleVersions(version, minRDBMSModelVersion) >= 0 {
			info["description"] = fmt.Sprintf("%v\n\nThe request body of 'POST /rdbms-model' and 'PUT /rdbms-model/{id}' follows the 'SourceRDBMS' / 'RecommendedRDBMS' struct of cm-beetle/imdl %s.", info["description"], version)
		}
		if compareModuleVersions(version, minObjectStorageModelVersion) >= 0 {
			info["description"] = fmt.Sprintf("%v\n\nThe request body of 'POST /object-storage-model' and 'PUT /object-storage-model/{id}' follows the 'SourceObjectStorage' / 'RecommendedObjectStorage' struct of cm-beetle/imdl %s.", info["description"], version)
		}
	}

	return marshalInTopLevelKeyOrder(spec, baseDoc)
}

// imdlPkgModelSwaggerInfo describes the APIs of a model in a package of cm-beetle/imdl other than the infra models
// (e.g. the RDBMS model in 'rdbms-model', and the object storage model in 'storage-model').
type imdlPkgModelSwaggerInfo struct {
	modelName    string               // the model name used in the notes (e.g. "RDBMS")
	minVersion   string               // the oldest cm-beetle/imdl version which has the package
	pkgDir       string               // the package directory in cm-beetle/imdl
	basePath     string               // the base path of the APIs (e.g. "/rdbms-model")
	reqDefName   string               // the definition of the request body (e.g. "handler.CreateRDBMSModelReq")
	versionParam string               // the query parameter of the model version (e.g. "rdbmsModelVersion")
	schemas      []pkgModelSchemaInfo // the structs of the body fields
}

var (
	rdbmsModelSwaggerInfo = imdlPkgModelSwaggerInfo{
		modelName:    "RDBMS",
		minVersion:   minRDBMSModelVersion,
		pkgDir:       rdbmsModelPkgDir,
		basePath:     "/rdbms-model",
		reqDefName:   "handler.CreateRDBMSModelReq",
		versionParam: "rdbmsModelVersion",
		schemas:      []pkgModelSchemaInfo{sourceRDBMSModelSchema, targetRDBMSModelSchema},
	}
	objectStorageModelSwaggerInfo = imdlPkgModelSwaggerInfo{
		modelName:    "object storage",
		minVersion:   minObjectStorageModelVersion,
		pkgDir:       objectStorageModelPkgDir,
		basePath:     "/object-storage-model",
		reqDefName:   "handler.CreateObjectStorageModelReq",
		versionParam: "objectStorageModelVersion",
		schemas:      []pkgModelSchemaInfo{sourceObjectStorageModelSchema, targetObjectStorageModelSchema},
	}
)

// addRDBMSModelToImdlSwaggerDoc changes the API definition of a cm-beetle/imdl version for the RDBMS model APIs:
// the body of CreateRDBMSModel / UpdateRDBMSModel follows the 'SourceRDBMS' / 'RecommendedRDBMS' struct of that version,
// and 'rdbmsModelVersion' of CreateRDBMSModel / ConvertRDBMSModel defaults to that version.
// A version older than minRDBMSModelVersion has no RDBMS model, so its definition notes that instead.
func addRDBMSModelToImdlSwaggerDoc(ctx context.Context, defs, paths map[string]interface{}, version string) error {
	return addPkgModelToImdlSwaggerDoc(ctx, defs, paths, version, rdbmsModelSwaggerInfo)
}

// addObjectStorageModelToImdlSwaggerDoc changes the API definition of a cm-beetle/imdl version for the object storage
// model APIs: the body of CreateObjectStorageModel / UpdateObjectStorageModel follows the 'SourceObjectStorage' / 'RecommendedObjectStorage'
// struct of that version, and 'objectStorageModelVersion' of CreateObjectStorageModel / ConvertObjectStorageModel defaults to that version.
// A version older than minObjectStorageModelVersion has no object storage model, so its definition notes that instead.
func addObjectStorageModelToImdlSwaggerDoc(ctx context.Context, defs, paths map[string]interface{}, version string) error {
	return addPkgModelToImdlSwaggerDoc(ctx, defs, paths, version, objectStorageModelSwaggerInfo)
}

// addPkgModelToImdlSwaggerDoc changes the API definition of a cm-beetle/imdl version for the APIs of the given model:
// the body of the create (POST) / update (PUT) APIs follows the structs of that version, and the model version parameter
// of the create / convert APIs defaults to that version.
func addPkgModelToImdlSwaggerDoc(ctx context.Context, defs, paths map[string]interface{}, version string, info imdlPkgModelSwaggerInfo) error {
	type operationRef struct{ path, method string }
	bodyOps := []operationRef{{info.basePath, "post"}, {info.basePath + "/{id}", "put"}}
	versionParamOps := []operationRef{{info.basePath, "post"}, {info.basePath + "/{id}/convert", "get"}}
	operationOf := func(op operationRef) map[string]interface{} {
		pathItem, _ := paths[op.path].(map[string]interface{})
		operation, _ := pathItem[op.method].(map[string]interface{})
		return operation
	}

	if compareModuleVersions(version, info.minVersion) < 0 {
		noModel := fmt.Sprintf("has no %s model", info.modelName)
		for _, op := range append(bodyOps, versionParamOps...) {
			operation := operationOf(op)
			if desc, ok := operation["description"].(string); ok && !strings.Contains(desc, noModel) {
				operation["description"] = desc + fmt.Sprintf("\n\n(cm-beetle/imdl %s %s. Use %s or later.)", version, noModel, info.minVersion)
			}
		}
		return nil
	}

	baseReqDef, ok := defs[info.reqDefName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("the API definition has no '%s'", info.reqDefName)
	}

	pkg, err := modelschema.LoadPackage(ctx, infraModelSchemaSource, version, info.pkgDir)
	if err != nil {
		return err
	}
	defPrefix := "imdl-" + version + "." + strings.ReplaceAll(info.pkgDir, "-", "") + "."
	reqDef := deepCopyJSON(baseReqDef).(map[string]interface{})
	props, _ := reqDef["properties"].(map[string]interface{})
	if props == nil {
		props = map[string]interface{}{}
		reqDef["properties"] = props
	}
	for _, schema := range info.schemas {
		if !pkg.HasType(schema.RootType) {
			props[schema.FieldName] = map[string]interface{}{
				"type":        "object",
				"description": fmt.Sprintf("'%s' struct does not exist in cm-beetle/imdl %s '%s'", schema.RootType, version, info.pkgDir),
			}
			continue
		}
		ref, err := pkg.AddSwaggerDefinitions(defs, schema.RootType, defPrefix)
		if err != nil {
			return err
		}
		props[schema.FieldName] = ref
	}
	reqDefName := "imdl-" + version + "." + info.reqDefName
	defs[reqDefName] = reqDef

	for _, op := range bodyOps {
		operation := operationOf(op)
		params, _ := operation["parameters"].([]interface{})
		for _, p := range params {
			if param, _ := p.(map[string]interface{}); param["in"] == "body" {
				param["schema"] = map[string]interface{}{"$ref": "#/definitions/" + reqDefName}
			}
		}
		if desc, ok := operation["description"].(string); ok {
			operation["description"] = desc + fmt.Sprintf("\n\n(This API definition shows the request body of cm-beetle/imdl %s.)", version)
		}
	}
	for _, op := range versionParamOps {
		params, _ := operationOf(op)["parameters"].([]interface{})
		for _, p := range params {
			if param, _ := p.(map[string]interface{}); param["in"] == "query" && param["name"] == info.versionParam {
				param["default"] = version
			}
		}
	}
	return nil
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
