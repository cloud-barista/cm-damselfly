package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"

	"path/filepath"

	"github.com/cloud-barista/cm-damselfly/pkg/config"
	"github.com/cloud-barista/cm-damselfly/pkg/lkvstore"
	"github.com/cloud-barista/cm-damselfly/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	restServer "github.com/cloud-barista/cm-damselfly/pkg/api/rest"
)

func init() {

	// Initialize the configuration from "config.yaml" file or environment variables
	config.Init()

	// Initialize the logger
	logger := logger.NewLogger(logger.Config{
		LogLevel:    config.Damselfly.LogLevel,
		LogWriter:   config.Damselfly.LogWriter,
		LogFilePath: config.Damselfly.LogFile.Path,
		MaxSize:     config.Damselfly.LogFile.MaxSize,
		MaxBackups:  config.Damselfly.LogFile.MaxBackups,
		MaxAge:      config.Damselfly.LogFile.MaxAge,
		Compress:    config.Damselfly.LogFile.Compress,
	})

	// Set a global logger
	log.Logger = *logger

	// Initialize the local key-value store with the specified file path
	dbFilePath := config.Damselfly.LKVStore.Path

	lkvstore.Init(lkvstore.Config{
		DbFilePath: dbFilePath,
	})

}

// @title CM-Damselfly REST API
// @version latest
// @description CM-Damselfly REST API

// @contact.name API Support
// @contact.url http://cloud-barista.github.io
// @contact.email contact-to-cloud-barista@googlegroups.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath /damselfly

// @securityDefinitions.basic BasicAuth

func main() {
	log.Info().Msg("Preparing to run CM-Damselfly")

	// Initialize the local key-value store with the specified file path
	dbFilePath := config.Damselfly.LKVStore.Path

	// Ensure the DB file directory exists before creating the log file
	dir := filepath.Dir(dbFilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Create the directory if it does not exist
		err = os.MkdirAll(dir, 0755) // Set permissions as needed
		if err != nil {
			log.Error().Msgf("Failed to Create the DB Directory: : [%v]", err)
		}
	}

	// Load the state from the file back into the key-value store
	if err := lkvstore.LoadLkvStore(); err != nil {
		fmt.Printf("Error loading: %v\n", err)
	} else {
		fmt.Println("Successfully loaded the lkvstore from file.")
	}

	defer func() {
		// Save the current state of the key-value store to file
		if err := lkvstore.SaveLkvStore(); err != nil {
			fmt.Printf("Error saving: %v\n", err)
		} else {
			fmt.Println("Successfully saved the lkvstore to file.")
		}
	}()

	log.Info().Msg("Setting mc-terrarium REST API server")

	// Set the default port number "8088" for the REST API server to listen on
	port := flag.String("port", "8088", "port number for the restapiserver to listen to")
	flag.Parse()

	// Validate port
	if portInt, err := strconv.Atoi(*port); err != nil || portInt < 1 || portInt > 65535 {
		log.Fatal().Msgf("%s is not a valid port number. Please retry with a valid port number (ex: -port=[1-65535]).", *port)
	}
	log.Debug().Msgf("port number: %s", *port)

	// Watch config file changes
	go func() {
		viper.WatchConfig()
		viper.OnConfigChange(func(e fsnotify.Event) {
			log.Debug().Str("file", e.Name).Msg("config file changed")
			err := viper.ReadInConfig()
			if err != nil { // Handle errors reading the config file
				log.Fatal().Err(err).Msg("fatal error in config file")
			}
			err = viper.Unmarshal(&config.RuntimeConfig)
			if err != nil {
				log.Panic().Err(err).Msg("error unmarshaling runtime configuration")
			}
			config.Damselfly = config.RuntimeConfig.Damselfly
		})
	}()

	// Launch API servers (REST)
	wg := new(sync.WaitGroup)
	wg.Add(1)

	log.Info().Msg("CM-Damselfly REST API server is starting...")
	// Start REST Server
	go func() {
		restServer.RunServer(*port)
		wg.Done()
	}()

	wg.Wait()
}
