package model

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
	// "github.com/davecgh/go-spew/spew"
)

const (
	ReplicationModelDir string = "/.replication_model"
)

type Replication struct {
	TemplateFormatVersion string     `yaml:"TemplateFormatVersion"`
	Description           string     `yaml:"Description"`
	CSP                   CSP        `yaml:"CSP"`
	Resources             Replica_RS `yaml:"Resources"`
}

type Replica_RS struct {
	VMInstance   VMInstance   `yaml:"VMInstance"`
	LoadBalancer LoadBalancer `yaml:"LoadBalancer"`
}

type CSP struct {
	Name       string `yaml:"Name"`
	RegionCode string `yaml:"Region"`
	ZoneCode   string `yaml:"Zone"`
}

type VMInstance struct {
	Name           string          `yaml:"Name"`
	ImageId        string          `yaml:"ImageId"`
	VMSpecId       string          `yaml:"VMSpecId"`
	KeyPairName    string          `yaml:"KeyPairName"`
	SecurityGroups []SecurityGroup `yaml:"SecurityGroups"`
	VPC            VPCInstance     `yaml:"VPCInstance"`
	Subnets        []Subnet        `yaml:"Subnets"`
}

type SecurityGroup struct {
	GroupDescription     string        `yaml:"GroupDescription"`
	SecurityGroupIngress []IngressRule `yaml:"SecurityGroupIngress"`
}

type VPCInstance struct {
	Name               string `yaml:"Name"`
	CidrBlock          string `yaml:"CidrBlock"`
	EnableDnsSupport   bool   `yaml:"EnableDnsSupport"`
	EnableDnsHostnames bool   `yaml:"EnableDnsHostnames"`
}

type Subnet struct {
	Name             string `yaml:"Name"`
	CidrBlock        string `yaml:"CidrBlock"`
	AvailabilityZone string `yaml:"AvailabilityZone"`
}

type IngressRule struct {
	IpProtocol string `yaml:"IpProtocol"`
	FromPort   string `yaml:"FromPort"`
	ToPort     string `yaml:"ToPort"`
	CidrIp     string `yaml:"CidrIp"`
}

type LoadBalancer struct {
	Name        string       `yaml:"Name"`
	Listeners   []Listener   `yaml:"Listeners"`
	HealthCheck HealthCheck  `yaml:"HealthCheck"`
	VMInstances []VMInstance `yaml:"VMInstances"`
}

type Listener struct {
	LoadBalancerPort string `yaml:"LoadBalancerPort"`
	InstancePort     string `yaml:"InstancePort"`
	Protocol         string `yaml:"Protocol"`
}

type HealthCheck struct {
	Target             string `yaml:"Target"`
	HealthyThreshold   string `yaml:"HealthyThreshold"`
	UnhealthyThreshold string `yaml:"UnhealthyThreshold"`
	Interval           string `yaml:"Interval"`
	Timeout            string `yaml:"Timeout"`
}

func GetReplicaResources(yamlName string) (Replication, error) {
	cblogger.Info("Model Handler called GetReplicaResources()!")

	modelFilePath := os.Getenv("MODEL_ROOT") + ReplicationModelDir + "/"
	yamlFile := modelFilePath + yamlName + ".yaml"
	fmt.Printf("# Replica YAML File: %+s\n", yamlFile)

	// Check if the Model file folder Exists, and Create it
	if err := CheckFolderAndCreate(modelFilePath); err != nil {
		newErr := fmt.Errorf("Failed to Create the Model Path : [%s] : [%v]"+modelFilePath, err)
		cblogger.Error(newErr.Error())
		return Replication{}, newErr
	}

	// Read YAML
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		log.Fatal(err)
	}
	// spew.Dump(data)

	// Unmarshal YAML
	var replica Replication
	err = yaml.Unmarshal(data, &replica)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = yaml.Unmarshal(data, &replica)
	// fmt.Printf("# Parsed Data: %+v\n", replica)
	return replica, nil
}
