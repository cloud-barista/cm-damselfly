package main

import (
	"flag"
	"strconv"
	"sync"

	"github.com/cloud-barista/cm-damselfly/pkg/config"
	"github.com/cloud-barista/cm-damselfly/pkg/logger"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	restServer "github.com/cloud-barista/cm-damselfly/pkg/api/rest"
)

func init() {

	config.Init()

	logger := logger.NewLogger(logger.Config{
		LogLevel:    viper.GetString("damselfly.log.level"),
		LogWriter:   viper.GetString("damselfly.log.writer"),
		LogFilePath: viper.GetString("damselfly.logfile.path"),
		MaxSize:     viper.GetInt("damselfly.logfile.maxsize"),
		MaxBackups:  viper.GetInt("damselfly.logfile.maxbackups"),
		MaxAge:      viper.GetInt("damselfly.logfile.maxage"),
		Compress:    viper.GetBool("damselfly.logfile.compress"),
	})

	log.Logger = *logger

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
	log.Info().Msg("CM-Damselfly server is starting...")

	// Set the default port number "8056" for the REST API server to listen on
	port := flag.String("port", "8088", "port number for the restapiserver to listen to")
	flag.Parse()

	// Validate port
	if portInt, err := strconv.Atoi(*port); err != nil || portInt < 1 || portInt > 65535 {
		log.Fatal().Msgf("%s is not a valid port number. Please retry with a valid port number (ex: -port=[1-65535]).", *port)
	}
	log.Debug().Msgf("port number: %s", *port)

	// Launch API servers (REST)
	wg := new(sync.WaitGroup)
	wg.Add(1)

	// Start REST Server
	go func() {
		restServer.RunServer(*port)
		wg.Done()
	}()

	wg.Wait()
}
