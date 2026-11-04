package handler

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
)

// [Note]
// A stored infra model is converted to a newer cm-beetle/imdl version in two steps:
//  1. The migrations between the stored version and the requested version move the renamed or restructured fields.
//     They apply only to the fields that actually exist, because the content of a stored model may be newer than its version.
//  2. The result is fitted to the struct of the requested version: the added fields are filled in, and the fields
//     which the struct does not declare (e.g. 'nameSeed', removed in v0.1.5) are removed and reported.
//
// A new tagged version which only adds fields needs no migration. If it renames a field, a migration must be added here,
// or the old field is removed and reported in 'droppedFields'.

// minCloudModelCurrentVersion is the oldest version with the 'RecommendedInfra' struct, which a cloud model can be converted to.
const minCloudModelCurrentVersion = "v0.1.2"

// maxGeneratedGpuCards limits the 'gpuCards' entries generated from the 'gpu.count' of a stored model.
const maxGeneratedGpuCards = 1024

// InfraModelConversion describes the result of converting an infra model to another model version.
type InfraModelConversion struct {
	OriginVersion  string   `json:"originVersion"`
	CurrentVersion string   `json:"currentVersion"`
	Converted      bool     `json:"converted"`
	DroppedFields  []string `json:"droppedFields"`
	Warnings       []string `json:"warnings"`
}

// infraModelMigration moves the fields changed in 'version'. It is applied if the model is older than 'version'.
// 'apply' changes the infra model in place and returns the warnings about what could not be converted as it was.
type infraModelMigration struct {
	version string
	apply   func(infraModel map[string]interface{}) []string
}

var (
	onpremInfraModelMigrations = []infraModelMigration{
		{version: "v0.1.4", apply: migrateServersToNodes},
		{version: "v0.1.15", apply: migrateGpuToGpuCards},
	}
	cloudInfraModelMigrations = []infraModelMigration{
		{version: "v0.1.1", apply: migrateSubGroupsToNodeGroups},
		{version: "v0.1.2", apply: migrateTargetVmFields},
		{version: "v0.1.11", apply: migratePostCommandToPostCommands},
	}
)

// validateInfraModelConversion checks that the stored model version can be converted to the requested version.
// Both versions must be tagged, and only the conversion to the same or a newer version is allowed.
func validateInfraModelConversion(isCloudModel bool, originVersion, currentVersion string, supportedVersions []string) error {
	if _, err := selectModelVersion(originVersion, supportedVersions); originVersion == "" || err != nil {
		return fmt.Errorf("the stored model version '%s' is not a cm-beetle/imdl tag, so the model cannot be converted", originVersion)
	}
	if isCloudModel && compareModuleVersions(currentVersion, minCloudModelCurrentVersion) < 0 {
		return fmt.Errorf("a cloud model cannot be converted to %s: the '%s' struct does not exist before %s", currentVersion, cloudInfraModelSchema.RootType, minCloudModelCurrentVersion)
	}
	if compareModuleVersions(currentVersion, originVersion) < 0 {
		return fmt.Errorf("the model version %s cannot be converted to the older version %s: only the conversion to a newer version is allowed", originVersion, currentVersion)
	}
	return nil
}

// convertInfraModel converts the raw infra model of 'originVersion' to the struct of 'currentVersion' in 'pkg'.
func convertInfraModel(pkg *modelschema.Package, isCloudModel bool, originVersion, currentVersion string, raw json.RawMessage) (json.RawMessage, InfraModelConversion, error) {
	schema, migrations := onpremInfraModelSchema, onpremInfraModelMigrations
	if isCloudModel {
		schema, migrations = cloudInfraModelSchema, cloudInfraModelMigrations
	}

	conversion := InfraModelConversion{
		OriginVersion:  originVersion,
		CurrentVersion: currentVersion,
		Converted:      true,
		DroppedFields:  []string{},
		Warnings:       []string{},
	}

	migrated, warnings, err := migrateInfraModel(migrations, originVersion, currentVersion, raw)
	if err != nil {
		return nil, conversion, err
	}
	conversion.Warnings = append(conversion.Warnings, warnings...)

	converted, dropped, err := pkg.NormalizeDroppingUnknown(migrated, schema.RootType, schema.FieldName)
	if err != nil {
		return nil, conversion, err
	}
	conversion.DroppedFields = append(conversion.DroppedFields, dropped...)
	return converted, conversion, nil
}

// migrateInfraModel applies the migrations of the versions in (originVersion, currentVersion], in order.
func migrateInfraModel(migrations []infraModelMigration, originVersion, currentVersion string, raw json.RawMessage) (json.RawMessage, []string, error) {
	// Numbers are kept as they are, and 'raw' is decoded into a new value, leaving the stored model unchanged.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var infraModel map[string]interface{}
	if err := dec.Decode(&infraModel); err != nil {
		return nil, nil, fmt.Errorf("the stored infra model is not a JSON object : [%v]", err)
	}

	var warnings []string
	for _, migration := range migrations {
		if compareModuleVersions(migration.version, originVersion) > 0 && compareModuleVersions(migration.version, currentVersion) <= 0 {
			warnings = append(warnings, migration.apply(infraModel)...)
		}
	}

	migrated, err := json.Marshal(infraModel)
	if err != nil {
		return nil, nil, err
	}
	return migrated, warnings, nil
}

// renameField moves a field to its new name. If the new name is already used, the old field is left in place,
// to be reported as a dropped field when the model is fitted to the struct of the requested version.
func renameField(object map[string]interface{}, from, to string) {
	value, ok := object[from]
	if !ok {
		return
	}
	if _, exists := object[to]; exists {
		return
	}
	delete(object, from)
	object[to] = value
}

// migrateServersToNodes applies the change of on-premise model v0.1.4: 'servers' -> 'nodes'.
func migrateServersToNodes(infraModel map[string]interface{}) []string {
	renameField(infraModel, "servers", "nodes")
	return nil
}

// gpuCommonFields are the fields of 'gpu' (the attributes shared by the devices) that each entry of 'gpuCards' has.
var gpuCommonFields = []string{"vendor", "model", "type", "driverVersion", "cudaVersion", "architecture"}

// gpuDetailFields maps the fields of 'gpu.details[]' to those of 'gpuCards[]', except for 'index'.
var gpuDetailFields = map[string]string{
	"uuid":        "uuid",
	"model":       "model",
	"pciBusId":    "pciBusId",
	"memoryTotal": "memoryTotalGB",
	"memoryFree":  "memoryFreeGB",
	"memoryUsed":  "memoryUsedGB",
}

// migrateGpuToGpuCards applies the change of on-premise model v0.1.15: 'nodes[].gpu' -> 'nodes[].gpuCards[]'.
func migrateGpuToGpuCards(infraModel map[string]interface{}) []string {
	var warnings []string
	nodes, _ := infraModel["nodes"].([]interface{})
	for i, item := range nodes {
		node, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		gpu, ok := node["gpu"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, exists := node["gpuCards"]; exists {
			continue
		}

		cards, cardWarnings := gpuCardsOf(gpu, fmt.Sprintf("nodes[%d].gpu", i))
		warnings = append(warnings, cardWarnings...)
		delete(node, "gpu")
		if len(cards) > 0 {
			node["gpuCards"] = cards
		}
	}
	return warnings
}

// gpuCardsOf builds the 'gpuCards' entries of a 'gpu' object: one per device of 'details',
// or 'count' entries with only the common attributes if it has no 'details'.
func gpuCardsOf(gpu map[string]interface{}, path string) ([]interface{}, []string) {
	var warnings []string

	common := map[string]interface{}{}
	for _, name := range gpuCommonFields {
		if value, ok := gpu[name]; ok && value != "" {
			common[name] = value
		}
	}
	newCard := func() map[string]interface{} {
		card := make(map[string]interface{}, len(common))
		for name, value := range common {
			card[name] = value
		}
		return card
	}

	var count int64
	if number, ok := gpu["count"].(json.Number); ok {
		count, _ = number.Int64()
	}
	details, _ := gpu["details"].([]interface{})

	var cards []interface{}
	hasCardMemory := false
	switch {
	case len(details) > 0:
		if count != 0 && count != int64(len(details)) {
			warnings = append(warnings, fmt.Sprintf("%s: 'count' (%d) differs from the number of 'details' (%d). One 'gpuCards' entry was created per 'details' entry.", path, count, len(details)))
		}
		for _, item := range details {
			detail, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			card := newCard()
			for from, to := range gpuDetailFields {
				if value, ok := detail[from]; ok && value != "" {
					card[to] = value
				}
			}
			if index, ok := detail["index"].(json.Number); ok {
				card["driverIndex"] = index.String()
			}
			if _, ok := card["memoryTotalGB"]; ok {
				hasCardMemory = true
			}
			cards = append(cards, card)
		}
	case count > maxGeneratedGpuCards:
		warnings = append(warnings, fmt.Sprintf("%s: 'count' (%d) exceeds %d, so no 'gpuCards' entries were created.", path, count, maxGeneratedGpuCards))
	case count > 0:
		for i := int64(0); i < count; i++ {
			cards = append(cards, newCard())
		}
		warnings = append(warnings, fmt.Sprintf("%s: there are no 'details', so %d 'gpuCards' entries were created with only the common attributes.", path, count))
	case len(common) > 0:
		warnings = append(warnings, fmt.Sprintf("%s: 'count' is 0 and there are no 'details', so no 'gpuCards' entries were created and the common attributes were not kept.", path))
	}

	if total, ok := gpu["totalMemoryGB"].(json.Number); ok && !hasCardMemory && total.String() != "0" {
		warnings = append(warnings, fmt.Sprintf("%s: 'totalMemoryGB' (%s) was not kept, because 'gpuCards' holds the memory per card and it cannot be divided among them.", path, total))
	}
	return cards, warnings
}

// migrateSubGroupsToNodeGroups applies the changes of cloud model v0.1.1: 'subGroups' -> 'nodeGroups' and the names of their fields.
func migrateSubGroupsToNodeGroups(infraModel map[string]interface{}) []string {
	if targetInfra, ok := infraModel["targetVmInfra"].(map[string]interface{}); ok {
		renameField(targetInfra, "subGroups", "nodeGroups")
		nodeGroups, _ := targetInfra["nodeGroups"].([]interface{})
		for _, item := range nodeGroups {
			if nodeGroup, ok := item.(map[string]interface{}); ok {
				renameField(nodeGroup, "subGroupSize", "nodeGroupSize")
				renameField(nodeGroup, "vmUserName", "nodeUserName")
				renameField(nodeGroup, "vmUserPassword", "nodeUserPassword")
			}
		}
	}

	images, _ := infraModel["targetVmOsImageList"].([]interface{})
	for _, item := range images {
		if image, ok := item.(map[string]interface{}); ok {
			renameField(image, "sourceVmUid", "sourceNodeUid")
		}
	}
	return nil
}

// migrateTargetVmFields applies the changes of cloud model v0.1.2: the 'Vm' in the names of the top-level fields was removed.
func migrateTargetVmFields(infraModel map[string]interface{}) []string {
	renameField(infraModel, "targetVmInfra", "targetInfra")
	renameField(infraModel, "targetVmOsImageList", "targetOsImageList")
	renameField(infraModel, "targetVmSpecList", "targetSpecList")
	return nil
}

// migratePostCommandToPostCommands applies the change of cloud model v0.1.11: 'targetInfra.postCommand' -> 'targetInfra.postCommands[]'.
func migratePostCommandToPostCommands(infraModel map[string]interface{}) []string {
	targetInfra, ok := infraModel["targetInfra"].(map[string]interface{})
	if !ok {
		return nil
	}
	postCommand, ok := targetInfra["postCommand"].(map[string]interface{})
	if !ok {
		return nil
	}
	if _, exists := targetInfra["postCommands"]; exists {
		return nil
	}

	delete(targetInfra, "postCommand")
	// 'postCommand' was not a pointer, so a model without a command to run has an empty one, which must not become a phase to run.
	if commands, _ := postCommand["command"].([]interface{}); hasNonEmptyString(commands) {
		targetInfra["postCommands"] = []interface{}{postCommand}
	}
	return nil
}

func hasNonEmptyString(values []interface{}) bool {
	for _, value := range values {
		if value != "" && value != nil {
			return true
		}
	}
	return false
}
