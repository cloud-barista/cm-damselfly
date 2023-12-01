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
    Description 	string `json:"description"`
    Name        	string `json:"name"`
    Version     	string `json:"version"`
	ModelId     	string `json:"modelId"`
	MigrationStep   string `json:"migrationStep"`
    TargetEnvironment TargetEnvironment `json:"targetEnvironment"`
}

type TargetEnvironment struct {
    Provider string 			`json:"provider"`
    Details  EnvironmentDetails `json:"details"`
}

type EnvironmentDetails struct {
    Region  string    `json:"region"`
    Zone    string    `json:"zone"`
    Resources []Resource `json:"resources"`
}

type Resource struct {
    Type                string             `json:"type"`
    NsSpecifications    *Namespace         `json:"nsSpecifications,omitempty"`
    VNetSpecifications  *VNet              `json:"vNetSpecifications,omitempty"`
    McisSpecifications  *MCIS              `json:"mcisSpecifications,omitempty"`
    ImgSpecifications   *VMImage           `json:"imgSpecifications,omitempty"`
    SgSpecifications    *SG     		   `json:"sgSpecifications,omitempty"`
    KeySpecifications   *KeyPair           `json:"keySpecifications,omitempty"`
    DbSpecifications    *Database          `json:"dbSpecifications,omitempty"`
    DiskSpecifications  *DataDisk          `json:"diskSpecifications,omitempty"`
    NlbSpecifications   *NLB               `json:"nlbSpecifications,omitempty"`
    AppSpecifications   *Application       `json:"appSpecifications,omitempty"`
    WebSvrSpecifications *WebServer        `json:"webSvrSpecifications,omitempty"`
}

// Define structs for each specification
type Namespace struct {
    Description string `json:"description"`
    Name        string `json:"name"`
}

type VNet struct {
    CidrBlock      string `json:"cidrBlock"`
    CspVNetId      string `json:"cspVNetId"`
    Description    string `json:"description"`
    Name           string `json:"name"`
    SubnetInfoList []SubnetInfo `json:"subnetInfoList"`
}

type MCIS struct {
    Description    string `json:"description"`
    InstallMonAgent string `json:"installMonAgent"`
    Label          string `json:"label"`
    Name           string `json:"name"`
    PlacementAlgo  string `json:"placementAlgo"`
    SystemLabel    string `json:"systemLabel"`
    VM             []VM   `json:"vm"`
}

type VMImage struct {
    CspImageId   string `json:"cspImageId"`
    Description  string `json:"description"`
    Name         string `json:"name"`
}

type SG struct {
    CspSecurityGroupId string         `json:"cspSecurityGroupId"`
    Description        string         `json:"description"`
    FirewallRules      []FirewallRule `json:"firewallRules"`
    Name               string         `json:"name"`
    VNetId             string         `json:"vNetId"`
}

type KeyPair struct {
    ConnectionName    string `json:"connectionName"`
    CspSshKeyId       string `json:"cspSshKeyId"`
    Description       string `json:"description"`
    Fingerprint       string `json:"fingerprint"`
    Name              string `json:"name"`
    PrivateKey        string `json:"privateKey"`
    PublicKey         string `json:"publicKey"`
    Username          string `json:"username"`
    VerifiedUsername  string `json:"verifiedUsername"`
}

type Database struct {
    DatabaseType    string `json:"databaseType"`
    Version         string `json:"version"`
    Size            string `json:"size"`
    Tables          string `json:"tables"`
    StoredProcedures string `json:"storedProcedures"`
}

type DataDisk struct {
    ConnectionName string `json:"connectionName"`
    CspDataDiskId  string `json:"cspDataDiskId"`
    Description    string `json:"description"`
    DiskSize       string `json:"diskSize"`
    DiskType       string `json:"diskType"`
    Name           string `json:"name"`
}

type NLB struct {
    CspNLBId       string      `json:"cspNLBId"`
    Description    string      `json:"description"`
    HealthChecker  HealthChecker `json:"healthChecker"`
    Listener       Listener    `json:"listener"`
    Scope          string      `json:"scope"`
    TargetGroup    TargetGroup `json:"targetGroup"`
    Type           string      `json:"type"`
}

type Application struct {
    Name         string   `json:"name"`
    Version      string   `json:"version"`
    Language     string   `json:"language"`
    Dependencies []string `json:"dependencies"`
}

type WebServer struct {
    Software       string `json:"software"`
    Version        string `json:"version"`
    HostedWebsites string `json:"hostedWebsites"`
    Traffic        string `json:"traffic"`
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
            modelInfo.MigrationModel.TargetEnvironment.Details.Resources[i].VNetSpecifications.SubnetInfoList = newSubnetInfoList
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
