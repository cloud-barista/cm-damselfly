package model

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	// "errors"

	cblog "github.com/cloud-barista/cb-log"
	"github.com/davecgh/go-spew/spew"
)

const (
	migrationModelDir string =  "/.migration_model"
)

func init() {
	// cblog is a global variable.
	cblogger = cblog.GetLogger("Model Handler")
}

type JSON_Model struct {
    MigrationModel MigrationModel `json:"migrationModel"`
}

type MigrationModel struct {
    Description string `json:"description"`
    Name        string `json:"name"`
    Version     string `json:"version"`
	ModelId     string `json:"modelId"`
    TargetEnvironment TargetEnvironment `json:"targetEnvironment"`
}

type TargetEnvironment struct {
    Provider string `json:"provider"`
    Details  Details `json:"details"`
}

type Details struct {
    Region  string    `json:"region"`
    Zone    string    `json:"zone"`
    Resources []Resource `json:"resources"`
}

type Resource struct {
    Type           string        `json:"type"`
    Name           string        `json:"name,omitempty"`
    Specifications Specifications `json:"specifications"`
}

type Specifications struct {
    // Common fields
    Description string `json:"description,omitempty"`
    Name        string `json:"name,omitempty"`

    // Fields specific to certain resources
    CidrBlock         string         `json:"cidrBlock,omitempty"`
    ConnectionName    string         `json:"connectionName,omitempty"`
    CspVNetId         string         `json:"cspVNetId,omitempty"`
    SubnetInfoList    []SubnetInfo   `json:"subnetInfoList,omitempty"`
    InstallMonAgent   string         `json:"installMonAgent,omitempty"`
    Label             string         `json:"label,omitempty"`
    PlacementAlgo     string         `json:"placementAlgo,omitempty"`
    SystemLabel       string         `json:"systemLabel,omitempty"`
    Vm                []VM           `json:"vm,omitempty"`
    CspImageId        string         `json:"cspImageId,omitempty"`
    CspSecurityGroupId string        `json:"cspSecurityGroupId,omitempty"`
    FirewallRules     []FirewallRule `json:"firewallRules,omitempty"`
    VNetId            string         `json:"vNetId,omitempty"`
    Fingerprint       string         `json:"fingerprint,omitempty"`
    PrivateKey        string         `json:"privateKey,omitempty"`
    PublicKey         string         `json:"publicKey,omitempty"`
    VerifiedUsername  string         `json:"verifiedUsername,omitempty"`
    DatabaseType      string         `json:"databaseType,omitempty"`
    Version           string         `json:"version,omitempty"`
    Size              string         `json:"size,omitempty"`
    Tables            string         `json:"tables,omitempty"`
    StoredProcedures  string         `json:"storedProcedures,omitempty"`
    DiskSize          string         `json:"diskSize,omitempty"`
    DiskType          string         `json:"diskType,omitempty"`
    CspNLBId          string         `json:"cspNLBId,omitempty"`
    HealthChecker     HealthChecker  `json:"healthChecker,omitempty"`
    LbListener        LbListener     `json:"listener,omitempty"`
    Scope             string         `json:"scope,omitempty"`
    TargetGroup       TargetGroup    `json:"targetGroup,omitempty"`
    Type              string         `json:"type,omitempty"`
    Language          string         `json:"language,omitempty"`
    Dependencies      []string       `json:"dependencies,omitempty"`
    Software          string         `json:"software,omitempty"`
    HostedWebsites    string         `json:"hostedWebsites,omitempty"`
    Traffic           string         `json:"traffic,omitempty"`
}

// Sub structs for nested objects in specifications
type SubnetInfo struct {
    Description string      `json:"description,omitempty"`
    Ipv4CIDR    string      `json:"ipv4_CIDR,omitempty"`
    KeyValueList []KeyValue `json:"keyValueList,omitempty"`
    Name        string      `json:"name,omitempty"`
}

type VM struct {
    DataDiskIds      []string    `json:"dataDiskIds,omitempty"`
	Description      string 	 `json:"description,omitempty"`
    IdByCsp          string      `json:"idByCsp,omitempty"`
    ImageId          string      `json:"imageId,omitempty"`
	Label            string 	 `json:"label,omitempty"`
	Name             string 	 `json:"name,omitempty"`
    RootDiskSize     string      `json:"rootDiskSize,omitempty"`
    RootDiskType     string      `json:"rootDiskType,omitempty"`
    SecurityGroupIds []string    `json:"securityGroupIds,omitempty"`
    SpecId           string      `json:"specId,omitempty"`
    SshKeyId         string      `json:"sshKeyId,omitempty"`
    SubGroupSize     string      `json:"subGroupSize,omitempty"`
    VNetId           string      `json:"vNetId,omitempty"`
    SubnetId         string      `json:"subnetId,omitempty"`
    VmUserAccount    string      `json:"vmUserAccount,omitempty"`
    VmUserPassword   string      `json:"vmUserPassword,omitempty"`
}

type FirewallRule struct {
    Cidr        string `json:"cidr,omitempty"`
    Direction   string `json:"direction,omitempty"`
    FromPort    string `json:"fromPort,omitempty"`
    IpProtocol  string `json:"ipprotocol,omitempty"`
    ToPort      string `json:"toPort,omitempty"`
}

type HealthChecker struct {
    Interval   string `json:"interval,omitempty"`
    Threshold  string `json:"threshold,omitempty"`
    Timeout    string `json:"timeout,omitempty"`
}

type LbListener struct {
    Port     string `json:"port,omitempty"`
    Protocol string `json:"protocol,omitempty"`
}

type TargetGroup struct {
    Port        string `json:"port,omitempty"`
    Protocol    string `json:"protocol,omitempty"`
    SubGroupId  string `json:"subGroupId,omitempty"`
}

type KeyValue struct {
    Key   string `json:"key,omitempty"`
    Value string `json:"value,omitempty"`
}

type IID struct {
	NameId   	string
	SystemId   	string
}

func WriteModel(modelName string, rsInfo JSON_Model) (JSON_Model, error){
	cblogger.Info("Model Handler called CreateModel()!")

	var modelIId IID
	modelIId.NameId = modelName
	// Check if the Model Name already Exists
	modelInfo, _ := GetModel(modelIId)
	if modelInfo.MigrationModel.Name != "" {
		newErr := fmt.Errorf("The Model Name already exists!!")
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	modelFilePath := os.Getenv("MODEL_ROOT") + migrationModelDir + "/"
	jsonFileName := modelFilePath + modelName + ".json"

	// Check if the Model file folder Exists, and Create it
	if err := CheckFolderAndCreate(modelFilePath); err != nil {
		newErr := fmt.Errorf("Failed to Create the Model Path : [%s] : [%v]" + modelFilePath, err)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	rsInfo.MigrationModel.Name = modelName
	file, err := json.MarshalIndent(rsInfo, "", " ")
	if err != nil {
		return JSON_Model{}, err
	}
	writeErr := os.WriteFile(jsonFileName, file, 0644)
	if writeErr != nil {
		newErr := fmt.Errorf("Failed to write the file : [%s] : [%v]", jsonFileName, writeErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	// Return the created Model Info.
	modelInfo, modelErr := GetModel(modelIId)
	if modelErr != nil {
		newErr := fmt.Errorf("Failed to Get the Model Info : [%v]", modelErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}
	return modelInfo, nil
}

func GetModel(modelIID IID) (JSON_Model, error) {
	cblogger.Info("Model Handler called GetModel()!")

	if !strings.EqualFold(modelIID.SystemId,"") {
		modelIID.NameId = modelIID.SystemId
    }

	modelFilePath := os.Getenv("MODEL_ROOT") + migrationModelDir + "/"
	jsonFileName := modelFilePath + modelIID.NameId + ".json"

	// Check if the Model file Folder Exists, and Create it
	if err := CheckFolderAndCreate(modelFilePath); err != nil {
		newErr := fmt.Errorf("Failed to Create the Model Path : [%s] : [%v]" + modelFilePath, err)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	file, err := os.ReadFile(jsonFileName)
	if err != nil {
		return JSON_Model{}, err
	}

	var rsData JSON_Model
	err = json.Unmarshal([]byte(file), &rsData)
	if err != nil {
		return JSON_Model{}, err
	}
	return rsData, nil
}

func ListModel() ([]*JSON_Model, error) {
	cblogger.Info("Model Handler : called ListModel()!")

	var modelIID IID
	var resourceInfoList []*JSON_Model

	medelFilePath := os.Getenv("MODEL_ROOT") + migrationModelDir + "/"
	dirFiles, readRrr := os.ReadDir(medelFilePath)
	if readRrr != nil {
		return nil, readRrr
	}

	spew.Dump(dirFiles)

	for _, file := range dirFiles {
		fileName := strings.TrimSuffix(file.Name(), ".json")  // 접미사 제거
		modelIID.NameId = fileName
		cblogger.Infof("# Model Name : " + modelIID.NameId)

		rsInfo, getErr := GetModel(modelIID)
		if getErr != nil {
			newErr := fmt.Errorf("Failed to Get the Model Info : [%v]", getErr)
			cblogger.Error(newErr.Error())
			return nil, newErr
		}		
		resourceInfoList = append(resourceInfoList, &rsInfo)
	}
	return resourceInfoList, nil
}

func UpdateModel(modelIId IID, rsInfo JSON_Model) (JSON_Model, error){
	cblogger.Info("Model Handler called UpdateModel()!")
	var modelName = modelIId.NameId

	// Find the created Model Info.
	modelInfo, modelErr := GetModel(modelIId)
	if modelErr != nil {
		newErr := fmt.Errorf("Failed to Find the Model Info!! : [%v]", modelErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	if modelInfo.MigrationModel.Name == "" {
		newErr := fmt.Errorf("Failed to Find the Model Info!!")
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	modelFilePath := os.Getenv("MODEL_ROOT") + migrationModelDir + "/"
	jsonFileName := modelFilePath + modelName + ".json"

	// Check if the Model file Folder Exists, and Create it
	if err := CheckFolderAndCreate(modelFilePath); err != nil {
		newErr := fmt.Errorf("Failed to Create the Model Path : [%s] : [%v]" + modelFilePath, err)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	rsInfo.MigrationModel.Name = modelName
	file, err := json.MarshalIndent(rsInfo, "", " ")
	if err != nil {
		return JSON_Model{}, err
	}

	writeErr := os.WriteFile(jsonFileName, file, 0644)
	if writeErr != nil {
		newErr := fmt.Errorf("Failed to write the file : [%s] : [%v]", jsonFileName, writeErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	// Return the updated Model Info.
	modelInfo, getErr := GetModel(modelIId)
	if getErr != nil {
		newErr := fmt.Errorf("Failed to Get the Model Info : [%v]", getErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}
	return modelInfo, nil
}

func DeleteModel(modelIID IID) (bool, error) {
	cblogger.Info("Model Handler called DelModel()!")

	if !strings.EqualFold(modelIID.SystemId,"") {
		modelIID.NameId = modelIID.SystemId
    }
	
	//To check whether the Model exists.
	_, modelErr := GetModel(modelIID)
	if modelErr != nil {
		newErr := fmt.Errorf("Failed to Get the Model Info : [%v]", modelErr)
		cblogger.Error(newErr.Error())
		return false, newErr
	}

	modelFilePath := os.Getenv("MODEL_ROOT") + migrationModelDir + "/"
	jsonFileName := modelFilePath + modelIID.NameId + ".json"

	// Check if the Model file Folder Exists, and Create it
	if err := CheckFolderAndCreate(modelFilePath); err != nil {
		newErr := fmt.Errorf("Failed to Create the Model Path : [%s] : [%v]" + modelFilePath, err)
		cblogger.Error(newErr.Error())
		return false, newErr
	}

	// To Remove the Model file on the Local machine.
	delErr := os.Remove(jsonFileName) 
	if delErr != nil {
		newErr := fmt.Errorf("Failed to Delete the Model : [%s] : [%v]", jsonFileName, delErr)
		cblogger.Error(newErr.Error())
		return false, newErr
	}
	return true, nil
}

// Function to update SubnetInfoList in the first vNet resource
func UpdateSubnetInfoList(modelIId IID, newSubnetInfoList []SubnetInfo) (JSON_Model, error){
	modelInfo, getErr := GetModel(modelIId)
	if getErr != nil {
		newErr := fmt.Errorf("Failed to Get the Model Info : [%v]", getErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}

	for i, resource := range modelInfo.MigrationModel.TargetEnvironment.Details.Resources {
		cblogger.Infof("# resource.Type : [%s]", resource.Type)
        if resource.Type == "vNet" {
            modelInfo.MigrationModel.TargetEnvironment.Details.Resources[i].Specifications.SubnetInfoList = newSubnetInfoList
			break
		}
    }
	
	_, err := UpdateModel(modelIId, modelInfo)
	if err != nil {
		newErr := fmt.Errorf("Failed to Update the Model Info : [%v]", err)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}
	// spew.Dump(model)
	
	// Return the updated Model Info.
	model, getErr := GetModel(modelIId)
	if getErr != nil {
		newErr := fmt.Errorf("Failed to Get the Model Info : [%v]", getErr)
		cblogger.Error(newErr.Error())
		return JSON_Model{}, newErr
	}
	return model, nil
}


// func SearchVMImageByName(rsInfo JSON_Model, imageName string) (*VMImage, error) {
// 	for _, image := range rsInfo.VMImages {
// 		if image.Name == imageName {
// 			return &image, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("VMImage not found")
// }

// func UpdateVMImageByName(rsInfo *JSON_Model, imageName string, newImage VMImage) error {
// 	for i, image := range rsInfo.VMImages {
// 		if image.Name == imageName {
// 			rsInfo.VMImages[i] = newImage
// 			return nil
// 		}
// 	}
// 	return fmt.Errorf("VMImage not found")
// }

// func DeleteVMImageByName(rsInfo *JSON_Model, imageName string) error {
// 	for i, image := range rsInfo.VMImages {
// 		if image.Name == imageName {
// 			rsInfo.VMImages = append(rsInfo.VMImages[:i], rsInfo.VMImages[i+1:]...)
// 			return nil
// 		}
// 	}
// 	return fmt.Errorf("VMImage not found")
// }

// func MergeJSONFiles(outputFile string, inputFiles ...string) error {
// 	mergedRS := JSON_Model{}

// 	for _, file := range inputFiles {
// 		data, err := os.ReadFile(file)
// 		if err != nil {
// 			return fmt.Errorf("error reading file %s: %v", file, err)
// 		}

// 		var rsInfo JSON_Model
// 		err = json.Unmarshal(data, &rsInfo)
// 		if err != nil {
// 			return fmt.Errorf("error unmarshalling file %s: %v", file, err)
// 		}

// 		mergedRS.VMImages = append(mergedRS.VMImages, rsInfo.VMImages...)
// 		mergedRS.VMSpecs = append(mergedRS.VMSpecs, rsInfo.VMSpecs...)
// 		//... similarly merge other resource lists
// 	}

// 	mergedData, err := json.MarshalIndent(mergedRS, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("error marshalling merged data: %v", err)
// 	}

// 	err = os.WriteFile(outputFile, mergedData, 0644)
// 	if err != nil {
// 		return fmt.Errorf("error writing to %s: %v", outputFile, err)
// 	}

// 	return nil
// }
