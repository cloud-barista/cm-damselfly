package model

import (
	"encoding/json"
	"sync"
	"os"
	"github.com/sirupsen/logrus"

	cblog "github.com/cloud-barista/cb-log"
)

var once sync.Once
var cblogger *logrus.Logger

// Cloud Object를 JSON String 타입으로 변환
func ConvertJsonString(v interface{}) (string, error) {
	jsonBytes, errJson := json.Marshal(v)

	if errJson != nil {
		cblogger.Error("Failed to Convert to JSON format.")
		cblogger.Error(errJson)
		return "", errJson
	}

	jsonString := string(jsonBytes)
	return jsonString, nil
}

func InitLog() {
	once.Do(func() {
		// cblog is a global variable.
		cblogger = cblog.GetLogger("Model Handler")
	})
}

// Check if the Folder Exists. If Not, Create it
func CheckFolderAndCreate(folderPath string) error {
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		if err := os.Mkdir(folderPath, 0700); err != nil {
			return err
		}
	}
	return nil
}
