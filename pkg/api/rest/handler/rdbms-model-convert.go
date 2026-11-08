package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
)

// [Note]
// A stored RDBMS model is converted to a newer cm-beetle/imdl version in the same way as an infra model (see infra-model-convert.go):
//  1. The migrations between the stored version and the requested version move the renamed or restructured fields.
//  2. The result is fitted to the struct of the requested version: the added fields are filled in, and the fields
//     which the struct does not declare are removed and reported in 'droppedFields'.
//
// The RDBMS model ('rdbms-model' package) exists since cm-beetle/imdl v0.1.13, and its struct was changed as follows.
//   - v0.1.14: 'SourceRDBMSProperty' (source and planning models) was restructured. The flat fields of an instance were
//     moved into 'dbNode' (the host: cpu, memory, disks) and 'dbEngine' (the engine software), and 'databases' was renamed
//     to 'innerDatabases'. Some fields (e.g. 'highAvailability') have no counterpart and are dropped.
//   - v0.1.17: 'characterSet' of 'TargetDatabase' (target models) was removed, and 'ncpDBACGToAllowAllInbound' was added.
//   - v0.1.18: 'charset' and 'collation' were added to 'TargetDatabase'.
// The other versions only add fields or keep the struct, so the conversion to the latest version needs no other migration.

// RDBMSModelConversion describes the result of converting an RDBMS model to another model version.
type RDBMSModelConversion struct {
	OriginVersion  string   `json:"originVersion"`
	CurrentVersion string   `json:"currentVersion"`
	Converted      bool     `json:"converted"`
	DroppedFields  []string `json:"droppedFields"`
	Warnings       []string `json:"warnings"`
}

// The RDBMS model migrations are applied like the infra model migrations: 'apply' changes the RDBMS model in place
// if the model is older than 'version' and the requested version is 'version' or newer.
var (
	sourceRDBMSModelMigrations = []infraModelMigration{
		{version: "v0.1.14", apply: migrateSourceRDBMSInstancesToDBNodeAndEngine},
	}
	targetRDBMSModelMigrations = []infraModelMigration{
		{version: "v0.1.18", apply: migrateTargetDatabaseCharacterSetToCharset},
	}
)

// rdbmsModelMigrationsOf returns the migrations of the RDBMS model struct used for the given model stage.
func rdbmsModelMigrationsOf(stage ModelStage) []infraModelMigration {
	if stage == StageTarget {
		return targetRDBMSModelMigrations
	}
	return sourceRDBMSModelMigrations
}

// newRDBMSModelConversion returns the conversion info without dropped fields or warnings.
func newRDBMSModelConversion(originVersion, currentVersion string, converted bool) RDBMSModelConversion {
	return RDBMSModelConversion{
		OriginVersion:  originVersion,
		CurrentVersion: currentVersion,
		Converted:      converted,
		DroppedFields:  []string{},
		Warnings:       []string{},
	}
}

// validateRDBMSModelConversion checks that the stored model version can be converted to the requested version.
// Both versions must be cm-beetle/imdl tags with the RDBMS model ('supportedVersions'), and only the conversion
// to the same or a newer version is allowed.
func validateRDBMSModelConversion(originVersion, currentVersion string, supportedVersions []string) error {
	if _, err := selectModelVersion(originVersion, supportedVersions); originVersion == "" || err != nil {
		return fmt.Errorf("the stored model version '%s' is not a cm-beetle/imdl tag with the RDBMS model (%s or later), so the model cannot be converted", originVersion, minRDBMSModelVersion)
	}
	if compareModuleVersions(currentVersion, originVersion) < 0 {
		return fmt.Errorf("the model version %s cannot be converted to the older version %s: only the conversion to a newer version is allowed", originVersion, currentVersion)
	}
	return nil
}

// convertRDBMSModel converts the raw RDBMS model of 'originVersion' to the struct of 'currentVersion' in 'pkg'.
// If the raw RDBMS model cannot be fitted to the struct, it returns a *modelschema.ValidationError.
func convertRDBMSModel(pkg *modelschema.Package, stage ModelStage, originVersion, currentVersion string, raw json.RawMessage) (json.RawMessage, RDBMSModelConversion, error) {
	schema := rdbmsModelSchemaOf(stage)
	conversion := newRDBMSModelConversion(originVersion, currentVersion, true)

	migrated, warnings, err := migrateInfraModel(rdbmsModelMigrationsOf(stage), originVersion, currentVersion, raw)
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

// objectsOf returns the JSON objects in the array 'key' of 'parent' (the other elements are skipped).
func objectsOf(parent map[string]interface{}, key string) []map[string]interface{} {
	items, _ := parent[key].([]interface{})
	objects := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]interface{}); ok {
			objects = append(objects, object)
		}
	}
	return objects
}

// childObject returns the object 'key' of 'parent', creating it if it does not exist.
// It returns nil if 'key' has a value which is not an object.
func childObject(parent map[string]interface{}, key string) map[string]interface{} {
	value, exists := parent[key]
	if !exists || value == nil {
		child := map[string]interface{}{}
		parent[key] = child
		return child
	}
	child, _ := value.(map[string]interface{})
	return child
}

// moveField moves the field 'from' of 'src' to the field 'to' of 'dst'. If 'dst' is nil or 'to' is already used,
// the field is left in place, to be reported as a dropped field when the model is fitted to the struct.
func moveField(src map[string]interface{}, from string, dst map[string]interface{}, to string) bool {
	value, ok := src[from]
	if !ok || dst == nil {
		return false
	}
	if _, exists := dst[to]; exists {
		return false
	}
	delete(src, from)
	dst[to] = value
	return true
}

// numberOf returns the value of a JSON number (json.Number when decoded with UseNumber, or float64).
func numberOf(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	}
	return 0, false
}

// isNonZeroJSONValue reports whether a JSON value carries information (not null, false, 0 or "").
func isNonZeroJSONValue(v interface{}) bool {
	switch value := v.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		return value != ""
	}
	if n, ok := numberOf(v); ok {
		return n != 0
	}
	return true
}

// sourceRDBMSFieldsRemovedInV0114 are the fields of 'SourceRDBMSProperty' up to v0.1.13 which have no counterpart in v0.1.14.
var sourceRDBMSFieldsRemovedInV0114 = []string{"highAvailability", "publicAccess", "iops", "nhnDBSGToAllowAllInbound", "backupRetentionDays"}

// migrateSourceRDBMSInstancesToDBNodeAndEngine applies the change of the RDBMS source model v0.1.14 to each of 'sourceRDBMSInstances':
//   - 'instanceName' -> 'displayName', and 'databases' -> 'innerDatabases'
//   - 'machineId' -> 'dbNode.machineId', and 'engine', 'engineVersion', 'port' -> 'dbEngine.*'
//   - 'vcpu' -> 'dbNode.cpu' (estimated as 1 CPU with 'vcpu' cores and threads)
//   - 'memoryMb' -> 'dbNode.memory.totalSize' (GiB, rounded; at least 1 GiB)
//   - 'storageSizeGb', 'storageType' -> 'dbNode.dataDisks[0].totalSize', '.type'
//
// The fields without a counterpart (e.g. 'highAvailability') are dropped, with a warning if they have a value.
// A field which already exists in the new form (the content may be newer than the stored version) is not overwritten.
func migrateSourceRDBMSInstancesToDBNodeAndEngine(rdbmsModel map[string]interface{}) []string {
	var warnings []string
	for i, instance := range objectsOf(rdbmsModel, "sourceRDBMSInstances") {
		path := fmt.Sprintf("%s.sourceRDBMSInstances[%d]", sourceRDBMSModelSchema.FieldName, i)

		renameField(instance, "instanceName", "displayName")
		renameField(instance, "databases", "innerDatabases")

		dbEngine := childObject(instance, "dbEngine")
		moveField(instance, "engine", dbEngine, "engine")
		moveField(instance, "engineVersion", dbEngine, "engineVersion")
		moveField(instance, "port", dbEngine, "port")

		dbNode := childObject(instance, "dbNode")
		moveField(instance, "machineId", dbNode, "machineId")
		if dbNode != nil {
			warnings = append(warnings, migrateVcpuToCPU(instance, dbNode, path)...)
			warnings = append(warnings, migrateMemoryMbToMemory(instance, dbNode, path)...)
			warnings = append(warnings, migrateStorageToDataDisk(instance, dbNode, path)...)
		}

		for _, name := range sourceRDBMSFieldsRemovedInV0114 {
			if value, exists := instance[name]; exists && isNonZeroJSONValue(value) {
				warnings = append(warnings, fmt.Sprintf("'%s.%s' (%v) has no corresponding field since v0.1.14, so it is dropped", path, name, value))
			}
		}
	}
	return warnings
}

// migrateVcpuToCPU moves 'vcpu' to 'dbNode.cpu', assuming 1 CPU (socket) with 'vcpu' cores and threads.
func migrateVcpuToCPU(instance, dbNode map[string]interface{}, path string) []string {
	value, exists := instance["vcpu"]
	if !exists {
		return nil
	}
	if _, used := dbNode["cpu"]; used {
		return nil
	}
	vcpu, ok := numberOf(value)
	if !ok || vcpu < 0 || vcpu != math.Trunc(vcpu) {
		return []string{fmt.Sprintf("'%s.vcpu' (%v) is not a number of vCPUs, so it is not converted to 'dbNode.cpu'", path, value)}
	}
	delete(instance, "vcpu")
	count := json.Number(strconv.FormatInt(int64(vcpu), 10))
	dbNode["cpu"] = map[string]interface{}{"cpus": json.Number("1"), "cores": count, "threads": count}
	return []string{fmt.Sprintf("'%s.vcpu' (%s) was converted to 'dbNode.cpu' as an estimate: cpus=1, cores=%s, threads=%s", path, count, count, count)}
}

// migrateMemoryMbToMemory moves 'memoryMb' (MB) to 'dbNode.memory.totalSize' (GiB).
func migrateMemoryMbToMemory(instance, dbNode map[string]interface{}, path string) []string {
	value, exists := instance["memoryMb"]
	if !exists {
		return nil
	}
	if _, used := dbNode["memory"]; used {
		return nil
	}
	memoryMb, ok := numberOf(value)
	if !ok || memoryMb < 0 {
		return []string{fmt.Sprintf("'%s.memoryMb' (%v) is not a memory size, so it is not converted to 'dbNode.memory'", path, value)}
	}
	delete(instance, "memoryMb")
	totalSize := int64(math.Round(memoryMb / 1024))
	if totalSize == 0 && memoryMb > 0 {
		totalSize = 1
	}
	dbNode["memory"] = map[string]interface{}{"totalSize": json.Number(strconv.FormatInt(totalSize, 10))}
	if float64(totalSize)*1024 != memoryMb {
		return []string{fmt.Sprintf("'%s.memoryMb' (%v MB) was converted to 'dbNode.memory.totalSize' (%d GiB) by rounding", path, value, totalSize)}
	}
	return nil
}

// migrateStorageToDataDisk moves 'storageSizeGb' and 'storageType' to the first entry of 'dbNode.dataDisks'.
func migrateStorageToDataDisk(instance, dbNode map[string]interface{}, path string) []string {
	_, hasSize := instance["storageSizeGb"]
	_, hasType := instance["storageType"]
	if !hasSize && !hasType {
		return nil
	}
	if _, used := dbNode["dataDisks"]; used {
		return nil
	}
	dataDisk := map[string]interface{}{}
	moveField(instance, "storageSizeGb", dataDisk, "totalSize")
	moveField(instance, "storageType", dataDisk, "type")
	dbNode["dataDisks"] = []interface{}{dataDisk}

	warnings := []string{fmt.Sprintf("'%s.storageSizeGb' and 'storageType' were converted to 'dbNode.dataDisks[0]' (the data disk of the DB node)", path)}
	if diskType, _ := dataDisk["type"].(string); diskType == "" {
		warnings = append(warnings, fmt.Sprintf("'%s.dbNode.dataDisks[0].type' is required but the stored model has no 'storageType'", path))
	}
	return warnings
}

// migrateTargetDatabaseCharacterSetToCharset applies the change of the RDBMS target model v0.1.18 to 'databases' of each of
// 'targetRDBMSInstances': 'characterSet' (removed in v0.1.17) -> 'charset'.
func migrateTargetDatabaseCharacterSetToCharset(rdbmsModel map[string]interface{}) []string {
	for _, instance := range objectsOf(rdbmsModel, "targetRDBMSInstances") {
		for _, database := range objectsOf(instance, "databases") {
			renameField(database, "characterSet", "charset")
		}
	}
	return nil
}
