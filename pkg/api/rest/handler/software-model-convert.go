package handler

import (
	"fmt"

	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
)

// [Note]
// A stored software model is converted to a newer cm-grasshopper/smdl version by fitting it to the struct of the requested version:
// the added fields are filled in, and the fields which the struct does not declare are removed and reported in 'droppedFields'.
//
// Up to v0.1.4, this is all that a conversion needs, because no field was renamed. The 'resources' of Kubernetes was changed from
// a free-form map to 'KubernetesResources' in v0.1.4, so its undeclared keys are dropped, and a declared key with a value of another
// type fails the conversion rather than losing the value silently. A new tagged version which renames a field needs a migration
// to be added here (see infra-model-convert.go), or the old field is removed and reported in 'droppedFields'.

// SoftwareModelConversion describes the result of converting a software model to another model version.
type SoftwareModelConversion struct {
	OriginVersion  string   `json:"originVersion"`
	CurrentVersion string   `json:"currentVersion"`
	Converted      bool     `json:"converted"`
	DroppedFields  []string `json:"droppedFields"`
	Warnings       []string `json:"warnings"`
}

// newSoftwareModelConversion returns the conversion info without dropped fields or warnings.
func newSoftwareModelConversion(originVersion, currentVersion string, converted bool) SoftwareModelConversion {
	return SoftwareModelConversion{
		OriginVersion:  originVersion,
		CurrentVersion: currentVersion,
		Converted:      converted,
		DroppedFields:  []string{},
		Warnings:       []string{},
	}
}

// validateSoftwareModelConversion checks that the stored model version can be converted to the requested version.
// Both versions must be tagged, and only the conversion to the same or a newer version is allowed.
func validateSoftwareModelConversion(originVersion, currentVersion string, supportedVersions []string) error {
	if _, err := selectModelVersion(originVersion, supportedVersions); originVersion == "" || err != nil {
		return fmt.Errorf("the stored model version '%s' is not a cm-grasshopper/smdl tag, so the model cannot be converted", originVersion)
	}
	if compareModuleVersions(currentVersion, originVersion) < 0 {
		return fmt.Errorf("the model version %s cannot be converted to the older version %s: only the conversion to a newer version is allowed", originVersion, currentVersion)
	}
	return nil
}

// convertSoftwareModel converts the raw software model of 'originVersion' to the struct of 'currentVersion' in 'pkg'.
// If the raw software model cannot be fitted to the struct, it returns a *modelschema.ValidationError.
func convertSoftwareModel(pkg *modelschema.Package, stage ModelStage, originVersion, currentVersion string, raw []byte) ([]byte, SoftwareModelConversion, error) {
	schema := softwareModelSchemaOf(stage)
	conversion := newSoftwareModelConversion(originVersion, currentVersion, true)

	converted, dropped, err := pkg.NormalizeDroppingUnknown(raw, schema.RootType, schema.FieldName)
	if err != nil {
		return nil, conversion, err
	}
	conversion.DroppedFields = append(conversion.DroppedFields, dropped...)
	return converted, conversion, nil
}
