package handler

import (
	"encoding/json"
	"fmt"

	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
)

// [Note]
// A stored object storage model is converted to a newer cm-beetle/imdl version in the same way as an infra model
// (see infra-model-convert.go):
//  1. The migrations between the stored version and the requested version move the renamed or restructured fields.
//  2. The result is fitted to the struct of the requested version: the added fields are filled in, and the fields
//     which the struct does not declare are removed and reported in 'droppedFields'.
//
// The object storage model ('storage-model' package) exists since cm-beetle/imdl v0.1.3, and its struct was changed as follows.
//   - v0.1.6: 'nameSeed' of 'RecommendedObjectStorage' (target models) was removed. (It is given as the 'nameSeed'
//     query parameter of the cm-beetle migration API instead.)
//   - v0.1.11: 'SourceObjectStorage' (source and planning models) was restructured. Up to v0.1.10, it described a single
//     bucket ('bucketName', 'versioningEnabled', ...). Since v0.1.11, it describes a group of buckets ('description',
//     'sourceCloud' and 'sourceObjectStorages'), and each bucket is a 'SourceObjectStorageProperty' with the former fields.
// The other versions (up to v0.1.20) only change the comments or keep the struct, so the conversion to the latest
// version needs no other migration.

// ObjectStorageModelConversion describes the result of converting an object storage model to another model version.
type ObjectStorageModelConversion struct {
	OriginVersion  string   `json:"originVersion"`
	CurrentVersion string   `json:"currentVersion"`
	Converted      bool     `json:"converted"`
	DroppedFields  []string `json:"droppedFields"`
	Warnings       []string `json:"warnings"`
}

// The object storage model migrations are applied like the infra model migrations: 'apply' changes the object storage
// model in place if the model is older than 'version' and the requested version is 'version' or newer.
var (
	sourceObjectStorageModelMigrations = []infraModelMigration{
		{version: "v0.1.11", apply: migrateSourceBucketToObjectStorageGroup},
	}
	targetObjectStorageModelMigrations = []infraModelMigration{
		{version: "v0.1.6", apply: warnTargetObjectStorageNameSeedRemoved},
	}
)

// objectStorageModelMigrationsOf returns the migrations of the object storage model struct used for the given model stage.
func objectStorageModelMigrationsOf(stage ModelStage) []infraModelMigration {
	if stage == StageTarget {
		return targetObjectStorageModelMigrations
	}
	return sourceObjectStorageModelMigrations
}

// newObjectStorageModelConversion returns the conversion info without dropped fields or warnings.
func newObjectStorageModelConversion(originVersion, currentVersion string, converted bool) ObjectStorageModelConversion {
	return ObjectStorageModelConversion{
		OriginVersion:  originVersion,
		CurrentVersion: currentVersion,
		Converted:      converted,
		DroppedFields:  []string{},
		Warnings:       []string{},
	}
}

// validateObjectStorageModelConversion checks that the stored model version can be converted to the requested version.
// Both versions must be cm-beetle/imdl tags with the object storage model ('supportedVersions'), and only the conversion
// to the same or a newer version is allowed.
func validateObjectStorageModelConversion(originVersion, currentVersion string, supportedVersions []string) error {
	if _, err := selectModelVersion(originVersion, supportedVersions); originVersion == "" || err != nil {
		return fmt.Errorf("the stored model version '%s' is not a cm-beetle/imdl tag with the object storage model (%s or later), so the model cannot be converted", originVersion, minObjectStorageModelVersion)
	}
	if compareModuleVersions(currentVersion, originVersion) < 0 {
		return fmt.Errorf("the model version %s cannot be converted to the older version %s: only the conversion to a newer version is allowed", originVersion, currentVersion)
	}
	return nil
}

// convertObjectStorageModel converts the raw object storage model of 'originVersion' to the struct of 'currentVersion' in 'pkg'.
// If the raw object storage model cannot be fitted to the struct, it returns a *modelschema.ValidationError.
func convertObjectStorageModel(pkg *modelschema.Package, stage ModelStage, originVersion, currentVersion string, raw json.RawMessage) (json.RawMessage, ObjectStorageModelConversion, error) {
	schema := objectStorageModelSchemaOf(stage)
	conversion := newObjectStorageModelConversion(originVersion, currentVersion, true)

	migrated, warnings, err := migrateInfraModel(objectStorageModelMigrationsOf(stage), originVersion, currentVersion, raw)
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

// sourceBucketFieldsUpToV0110 are the fields of 'SourceObjectStorage' up to v0.1.10 (a single bucket), which are the fields
// of 'SourceObjectStorageProperty' (an entry of 'sourceObjectStorages') since v0.1.11.
var sourceBucketFieldsUpToV0110 = []string{
	"bucketName",
	"versioningEnabled", "corsEnabled", "corsRule", "encryptionEnabled", "isPublic", // BucketFeatureProperty
	"totalSizeBytes", "objectCount", "accessFrequency", // BucketUsageProperty
	"tags", "creationDate", // BucketMetaProperty
}

// migrateSourceBucketToObjectStorageGroup applies the change of the object storage source model v0.1.11: the fields of
// the single bucket (up to v0.1.10) are moved into the first entry of 'sourceObjectStorages'.
//
// If the model already has 'sourceObjectStorages' (the content may be newer than the stored version), it is not changed,
// and the fields of the single bucket are left in place, to be reported as dropped fields.
func migrateSourceBucketToObjectStorageGroup(storageModel map[string]interface{}) []string {
	if _, exists := storageModel["sourceObjectStorages"]; exists {
		return nil
	}

	bucket := map[string]interface{}{}
	for _, name := range sourceBucketFieldsUpToV0110 {
		moveField(storageModel, name, bucket, name)
	}
	if len(bucket) == 0 {
		return nil
	}
	storageModel["sourceObjectStorages"] = []interface{}{bucket}

	name := sourceObjectStorageModelSchema.FieldName
	return []string{fmt.Sprintf("'%s' (a single bucket up to v0.1.10) was converted to '%s.sourceObjectStorages[0]' (a group of buckets since v0.1.11)", name, name)}
}

// warnTargetObjectStorageNameSeedRemoved reports the 'nameSeed' of the object storage target model, which was removed in v0.1.6.
// The field itself is left in place, to be reported as a dropped field.
func warnTargetObjectStorageNameSeedRemoved(storageModel map[string]interface{}) []string {
	value, exists := storageModel["nameSeed"]
	if !exists || !isNonZeroJSONValue(value) {
		return nil
	}
	return []string{fmt.Sprintf("'%s.nameSeed' (%v) has no corresponding field since v0.1.6, so it is dropped. Give it as the 'nameSeed' query parameter of the cm-beetle object storage migration API instead",
		targetObjectStorageModelSchema.FieldName, value)}
}
