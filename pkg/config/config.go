package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var (
	RuntimeConfig Config
	Damselfly     DamselflyConfig
)

type Config struct {
	Damselfly DamselflyConfig `mapstructure:"damselfly"`
}

type DamselflyConfig struct {
	Root        string            `mapstructure:"root"`
	Self        SelfConfig        `mapstructure:"self"`
	API         ApiConfig         `mapstructure:"api"`
	LKVStore    LkvStoreConfig    `mapstructure:"lkvstore"`
	LogFile     LogfileConfig     `mapstructure:"logfile"`
	LogLevel    string            `mapstructure:"loglevel"`
	LogWriter   string            `mapstructure:"logwriter"`
	Node        NodeConfig        `mapstructure:"node"`
	AutoControl AutoControlConfig `mapstructure:"autocontrol"`
}

type SelfConfig struct {
	Endpoint string `mapstructure:"endpoint"`
}

type ApiConfig struct {
	Allow    AllowConfig `mapstructure:"allow"`
	Auth     AuthConfig  `mapstructure:"auth"`
	Username string      `mapstructure:"username"`
	Password string      `mapstructure:"password"`
}

type AllowConfig struct {
	Origins string `mapstructure:"origins"`
}
type AuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type LkvStoreConfig struct {
	Path string `mapstructure:"path"`
}

type LogfileConfig struct {
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"maxsize"`
	MaxBackups int    `mapstructure:"maxbackups"`
	MaxAge     int    `mapstructure:"maxage"`
	Compress   bool   `mapstructure:"compress"`
}

type NodeConfig struct {
	Env string `mapstructure:"env"`
}

type AutoControlConfig struct {
	DurationMilliSec int `mapstructure:"duration_ms"`
}

func Init() {
	viper.AddConfigPath("../../conf/") // config for development
	viper.AddConfigPath(".")           // config for production optionally looking for the configuration in the working directory
	viper.AddConfigPath("./conf/")     // config for production optionally looking for the configuration in the working directory/conf/
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("no main config file, using default settings: %s\n", err)
		log.Printf("no main config file, using default settings: %s", err)
	}

	// Explicitly bind environment variables to configuration keys
	bindEnvironmentVariables()

	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	if viper.GetString("damselfly.root") == "" {
		log.Println("Finding project root by using project name")

		projectRoot := findProjectRoot("cm-damselfly")
		viper.Set("damselfly.root", projectRoot)
	}

	if err := viper.Unmarshal(&RuntimeConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
	Damselfly = RuntimeConfig.Damselfly

	// Print settings if in development mode
	if Damselfly.Node.Env == "development" {
		settings := viper.AllSettings()
		recursivePrintMap(settings, "")
	}
}

// NVL is func for null value logic
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}

func findProjectRoot(projectName string) string {
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}
	execDir := filepath.Dir(execPath)
	projectRoot, err := checkProjectRootInParentDirectory(projectName, execDir)
	if err != nil {
		fmt.Printf("Set current directory as project root directory (%v)\n", err)
		log.Printf("Set current directory as project root directory (%v)", err)
		projectRoot = execDir
	}
	fmt.Printf("Project root directory: %s\n", projectRoot)
	log.Printf("Project root directory: %s\n", projectRoot)
	return projectRoot
}

func checkProjectRootInParentDirectory(projectName string, execDir string) (string, error) {

	// Append a path separator to the project name for accurate matching
	projectNameWithSeparator := projectName + string(filepath.Separator)
	// Find the last index of the project name with the separator
	index := strings.LastIndex(execDir, projectNameWithSeparator)
	if index == -1 {
		return "", errors.New("project name not found in the path")
	}

	// Cut the string up to the index + length of the project name
	result := execDir[:index+len(projectNameWithSeparator)-1]

	return result, nil
}

func recursivePrintMap(m map[string]interface{}, prefix string) {
	for k, v := range m {
		fullKey := prefix + k
		if nestedMap, ok := v.(map[string]interface{}); ok {
			// Recursive call for nested maps
			recursivePrintMap(nestedMap, fullKey+".")
		} else {
			// Print current key-value pair
			log.Printf("Key: %s, Value: %v\n", fullKey, v)
		}
	}
}

func bindEnvironmentVariables() {
	// Explicitly bind environment variables to configuration keys
	viper.BindEnv("damselfly.root", "DAMSELFLY_ROOT")
	viper.BindEnv("damselfly.self.endpoint", "DAMSELFLY_SELF_ENDPOINT")
	viper.BindEnv("damselfly.api.allow.origins", "DAMSELFLY_API_ALLOW_ORIGINS")
	viper.BindEnv("damselfly.api.auth.enabled", "DAMSELFLY_API_AUTH_ENABLED")
	viper.BindEnv("damselfly.api.username", "DAMSELFLY_API_USERNAME")
	viper.BindEnv("damselfly.api.password", "DAMSELFLY_API_PASSWORD")
	viper.BindEnv("damselfly.lkvstore.path", "DAMSELFLY_LKVSTORE_PATH")
	viper.BindEnv("damselfly.logfile.path", "DAMSELFLY_LOGFILE_PATH")
	viper.BindEnv("damselfly.logfile.maxsize", "DAMSELFLY_LOGFILE_MAXSIZE")
	viper.BindEnv("damselfly.logfile.maxbackups", "DAMSELFLY_LOGFILE_MAXBACKUPS")
	viper.BindEnv("damselfly.logfile.maxage", "DAMSELFLY_LOGFILE_MAXAGE")
	viper.BindEnv("damselfly.logfile.compress", "DAMSELFLY_LOGFILE_COMPRESS")
	viper.BindEnv("damselfly.loglevel", "DAMSELFLY_LOGLEVEL")
	viper.BindEnv("damselfly.logwriter", "DAMSELFLY_LOGWRITER")
	viper.BindEnv("damselfly.node.env", "DAMSELFLY_NODE_ENV")
	viper.BindEnv("damselfly.autocontrol.duration_ms", "DAMSELFLY_AUTOCONTROL_DURATION_MS")
}
