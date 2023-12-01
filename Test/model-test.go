package main

import (
	// "encoding/json"
	// // "os"
	// "strings"
	// "log"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"

	cblog "github.com/cloud-barista/cb-log"
	model "github.com/cloud-barista/cm-damselfly"
)

var cblogger *logrus.Logger

func init() {
	// cblog is a global variable.
	cblogger = cblog.GetLogger("Migration Model Test")
	cblog.SetLevel("info")
}

func handleModel() {
	cblogger.Debug("Start Migration Model Test")

	var modelIId model.IID
	modelIId.NameId = "migrationModel-test-1"

	for {
		fmt.Println("\n============================================================================================")
		fmt.Println("[ Migration Model Test ]")
		fmt.Println("1. WriteModel()")
		fmt.Println("2. GetModel()")
		fmt.Println("3. ListModel()")
		fmt.Println("4. UpdateModel()")
		fmt.Println("5. DeleteModel()")
		fmt.Println("6. UpdateSubnetInfoList()")
		fmt.Println("0. Exit")
		fmt.Println("\n   Select a number above!! : ")
		fmt.Println("============================================================================================")


		var commandNum int
		inputCnt, err := fmt.Scan(&commandNum)
		if err != nil {
			panic(err)
		}

		if inputCnt == 1 {
			switch commandNum {
			case 1:
				fmt.Println("Start WriteModel() ...")

				var jsonModel model.JSON_Model
				jsonModel = model.JSON_Model {
					model.MigrationModel{
						Description: "Description for this model",
						Name:        "My-New-Model",
						Version:     "v0.1",
						ModelId:     "Otacbatcvafafa001",
						MigrationStep: "Step-01-abcdefg",
						TargetEnvironment: model.TargetEnvironment{
							Provider: "NCP VPC",
							Details: model.EnvironmentDetails{
								Region: "Korea",
								Zone:   "KR-1",
								Resources: []model.Resource{
									{
										Type: "Namespace",
										NsSpecifications: &model.Namespace{
											Description: "MyVPC_001",
											Name:        "ns01",
										},
									},
									{
										Type: "vNet",
										VNetSpecifications: &model.VNet{							
											CidrBlock:      "string",
											SubnetInfoList : []model.SubnetInfo{
												{
													Description: "New Subnet Description",
													Ipv4CIDR:    "192.168.1.0/24",
													KeyValueList: []model.KeyValue{
														{
															Key:   "NewKey",
															Value: "NewValue",
														},
													},
													Name: "NewSubnetName",
												},
											},
										},
									},
									{
										Type: "MCIS",
										McisSpecifications: &model.MCIS{
											Description:      "MCIS descrition ...",
											InstallMonAgent:  "no",
											Label:            "custom tag",
											Name:             "mcis01",
											PlacementAlgo:    "string",
											SystemLabel:      "",
											VM: []model.VM{
												{
													DataDiskIds:      []string{"disk1", "disk2"},
													Description:      "Description",
													IdByCsp:          "i-014fa6ede6ada0b2c",
													ImageId:          "image1",
													Label:            "label1",
													Name:             "g1-1",
													RootDiskSize:     "42",
													RootDiskType:     "SSD",
													SecurityGroupIds: []string{"sg1", "sg2"},
													SpecId:           "spec1",
													SshKeyId:         "key1",
													SubGroupSize:     "3",
													VNetId:           "vnet1",
													SubnetId:         "subnet1",
													VmUserAccount:    "user1",
													VmUserPassword:   "pass1",
												},
											},
										},
									},
								},
							},
						},
					},
				}
				
				modelInfo, err := model.WriteModel(modelIId.NameId, jsonModel)
				if err != nil {
					cblogger.Error(err)
					cblogger.Error("Model 쓰기 실패 : ", err)
				} else {
					fmt.Println("\n==================================================================================================================")
					cblogger.Debug("Model 쓰기 성공!!")
					spew.Dump(modelInfo)
				}
				fmt.Println("\nWriteModel Test Finished")

			case 2:
				fmt.Println("Start GetModel() ...")
				modelInfo, err := model.GetModel(modelIId)
				if err != nil {
					cblogger.Error(err)
					cblogger.Error("Model Info 조회 실패 : ", err)
				} else {
					fmt.Println("\n==================================================================================================================")
					spew.Dump(modelInfo)
					cblogger.Debug(modelInfo)
					//cblogger.Infof(modelInfo)
				}
				fmt.Println("\nGetModel() Test Finished")

			case 3:
				fmt.Println("Start ListModel() ...")				
				modelList, err := model.ListModel()
				if err != nil {
					cblogger.Error(err)
					cblogger.Error("Model list 조회 실패 : ", err)
				} else {
					cblogger.Debug("Model list 조회 성공")
					spew.Dump(modelList)
					cblogger.Debug(modelList)
					//spew.Dump(result)
					//fmt.Println(result)
					//fmt.Println("=========================")
					//fmt.Println(result)
					cblogger.Infof("List 개수 : [%d]", len(modelList))
				}
				fmt.Println("\nListModel() Test Finished")

			case 4:
				fmt.Println("Start UpdateModel() ...")

				var jsonModel2 model.JSON_Model
				jsonModel2 = model.JSON_Model {
					model.MigrationModel{
						Description: "Description for this model",
						Name:        "My-New-Model",
						Version:     "v0.2",
						ModelId:     "Otacbatcvafafa00100000000",
						MigrationStep: "Step-02-abcdefg",
						TargetEnvironment: model.TargetEnvironment{
							Provider: "AWS",
							Details: model.EnvironmentDetails{
								Region: "Korea",
								Zone:   "KR-1",
								Resources: []model.Resource{
									{
										Type: "Namespace",
										NsSpecifications: &model.Namespace{
											Description: "MyVPC_001",
											Name:        "ns01",
										},
									},
									{
										Type: "vNet",
										VNetSpecifications: &model.VNet{							
											CidrBlock:      "string",
											SubnetInfoList : []model.SubnetInfo{
												{
													Description: "New Subnet Description",
													Ipv4CIDR:    "10.10.1.0/24",
													KeyValueList: []model.KeyValue{
														{
															Key:   "NewKey",
															Value: "NewValue",
														},
													},
													Name: "NewSubnetName",
												},
											},
										},
									},
									{
										Type: "MCIS",
										McisSpecifications: &model.MCIS{
											Description:      "MCIS descrition ...",
											InstallMonAgent:  "no",
											Label:            "custom tag",
											Name:             "mcis01",
											PlacementAlgo:    "string",
											SystemLabel:      "",
											VM: []model.VM{
												{
													DataDiskIds:      []string{"disk1", "disk2"},
													Description:      "Description",
													IdByCsp:          "i-014fa6ede6ada0b2c",
													ImageId:          "image1",
													Label:            "label1",
													Name:             "g1-1",
													RootDiskSize:     "42",
													RootDiskType:     "SSD",
													SecurityGroupIds: []string{"sg1", "sg2"},
													SpecId:           "spec1",
													SshKeyId:         "key1",
													SubGroupSize:     "3",
													VNetId:           "vnet1",
													SubnetId:         "subnet1",
													VmUserAccount:    "user1",
													VmUserPassword:   "pass1",
												},
											},
										},
									},
								},
							},
						},
					},
				}

				modelInfo, err := model.UpdateModel(modelIId, jsonModel2)
				if err != nil {
					cblogger.Error(err)
					cblogger.Error("Update Model 실패 : ", err)
				} else {
					cblogger.Debugf("Update Model 성공")
					spew.Dump(modelInfo)
					cblogger.Debug(modelInfo)
					//fmt.Println(modelInfo)
				}
				fmt.Println("\nUpdateModel() Test Finished")

			case 5:
				fmt.Println("Start DeleteModel() ...")				
				result, err := model.DeleteModel(modelIId)
				if err != nil {
					cblogger.Error(err)
					cblogger.Error("DeleteModel 실패 : ", err)
				} else {
					cblogger.Debug("DeleteModel 성공")
					spew.Dump(result)
					cblogger.Debug(result)
					//spew.Dump(result)
					//fmt.Println(result)
					//fmt.Println("=========================")
					//fmt.Println(result)
				}
				fmt.Println("\nDeleteModel() Test Finished")	

			case 6:
				fmt.Println("Start UpdateSubnetInfoList() ...")

				newSubnetInfoList := []model.SubnetInfo{
					{
						Description: "New Subnet Description",
						Ipv4CIDR:    "10.10.2.0/24",
						KeyValueList: []model.KeyValue{
							{
								Key:   "NewKey",
								Value: "NewValue",
							},
						},
						Name: "NewSubnetName",
					},
				}

				modelInfo, err := model.UpdateSubnetInfoList(modelIId, newSubnetInfoList)
				if err != nil {
					cblogger.Error(err)
					cblogger.Error("UpdateSubnetInfoList 실패 : ", err)
				} else {
					cblogger.Debug("UpdateSubnetInfoList 성공")
					spew.Dump(modelInfo)
					cblogger.Debug(modelInfo)
					//fmt.Println(result)
					//fmt.Println("=========================")
					//fmt.Println(result)
				}
				fmt.Println("\nUpdateSubnetInfoList() Test Finished")

			case 0:
				fmt.Println("Exit")
				return
			}
		}
	}
}

func main() {
	cblogger.Info("Migration Model Test")
	handleModel()
}
