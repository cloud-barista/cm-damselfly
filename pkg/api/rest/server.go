/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package server is to handle REST API
package server

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloud-barista/cm-damselfly/pkg/api/rest/handler"
	mw "github.com/cloud-barista/cm-damselfly/pkg/api/rest/middleware"
	"github.com/cloud-barista/cm-damselfly/pkg/common"
	"github.com/cloud-barista/cm-damselfly/pkg/config"

	"crypto/subtle"
	"fmt"
	"os"

	"net/http"

	// REST API (echo)
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// echo-swagger middleware
	_ "github.com/cloud-barista/cm-damselfly/api"
	echoSwagger "github.com/swaggo/echo-swagger"

	// Black import (_) is for running a package's init() function without using its other contents.
	"github.com/rs/zerolog/log"
)

//var masterConfigInfos confighandler.MASTERCONFIGTYPE

const (
	infoColor    = "\033[1;34m%s\033[0m"
	noticeColor  = "\033[1;36m%s\033[0m"
	warningColor = "\033[1;33m%s\033[0m"
	errorColor   = "\033[1;31m%s\033[0m"
	debugColor   = "\033[0;36m%s\033[0m"
)

const (
	website = " https://github.com/cloud-barista/cm-damselfly"
	banner  = `    

 ██████╗ ███████╗ █████╗ ██████╗ ██╗   ██╗
 ██╔══██╗██╔════╝██╔══██╗██╔══██╗╚██╗ ██╔╝
 ██████╔╝█████╗  ███████║██║  ██║ ╚████╔╝ 
 ██╔══██╗██╔══╝  ██╔══██║██║  ██║  ╚██╔╝  
 ██║  ██║███████╗██║  ██║██████╔╝   ██║   
 ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝    ╚═╝   

 ██████╗  █████╗ ███╗   ███╗███████╗███████╗██╗     ███████╗██╗  ██╗   ██╗
 ██╔══██╗██╔══██╗████╗ ████║██╔════╝██╔════╝██║     ██╔════╝██║  ╚██╗ ██╔╝
 ██║  ██║███████║██╔████╔██║███████╗█████╗  ██║     █████╗  ██║   ╚████╔╝ 
 ██║  ██║██╔══██║██║╚██╔╝██║╚════██║██╔══╝  ██║     ██╔══╝  ██║    ╚██╔╝  
 ██████╔╝██║  ██║██║ ╚═╝ ██║███████║███████╗███████╗██║     ███████╗██║   
 ╚═════╝ ╚═╝  ╚═╝╚═╝     ╚═╝╚══════╝╚══════╝╚══════╝╚═╝     ╚══════╝╚═╝   

 Cloud Migration Model
 ________________________________________________`
)

// Created by https://patorjk.com/software/taag/#p=display&f=ANSI%20Shadow&t=Damselfly

// RunServer func start Rest API server
func RunServer(port string) {

	log.Info().Msg("CM-Damselfly REST API server is starting...")

	e := echo.New()

	// Middleware
	// e.Use(middleware.Logger()) // default logger middleware in echo

	APILogSkipPatterns := [][]string{
		{"/damselfly/api"},
		// {"/mcis", "option=status"},
	}

	// Custom logger middleware with zerolog
	e.Use(mw.Zerologger(APILogSkipPatterns))

	e.Use(middleware.Recover())
	// limit the application to 20 requests/sec using the default in-memory store
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))

	e.HideBanner = true
	//e.colorer.Printf(banner, e.colorer.Red("v"+Version), e.colorer.Blue(website))

	allowedOrigins := config.Damselfly.API.Allow.Origins
	if allowedOrigins == "" {
		log.Fatal().Msg("allow_ORIGINS env variable for CORS is " + allowedOrigins +
			". Please provide a proper value and source setup.env again. EXITING...")
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{allowedOrigins},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Conditions to prevent abnormal operation due to typos (e.g., ture, falss, etc.)
	enableAuth := config.Damselfly.API.Auth.Enabled

	apiUser := config.Damselfly.API.Username
	apiPass := config.Damselfly.API.Password

	if enableAuth {
		e.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
			// Skip authentication for some routes that do not require authentication
			Skipper: func(c echo.Context) bool {
				if c.Path() == "/damselfly/readyz" ||
					c.Path() == "/damselfly/httpVersion" {
					return true
				}
				return false
			},
			Validator: func(username, password string, c echo.Context) (bool, error) {
				// Be careful to use constant time comparison to prevent timing attacks
				if subtle.ConstantTimeCompare([]byte(username), []byte(apiUser)) == 1 &&
					subtle.ConstantTimeCompare([]byte(password), []byte(apiPass)) == 1 {
					return true, nil
				}
				return false, nil
			},
		}))
	}

	fmt.Println("\n \n ")
	fmt.Print(banner)
	fmt.Println("\n ")
	fmt.Println("\n ")
	fmt.Printf(infoColor, website)
	fmt.Println("\n \n ")

	// Route for system management
	// e.GET("/damselfly/swagger/*", echoSwagger.WrapHandler)
	// e.GET("/damselfly/swaggerActive", rest_common.RestGetSwagger)
	swaggerRedirect := func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/damselfly/api/index.html")
	}
	e.GET("/damselfly/api", swaggerRedirect)
	e.GET("/damselfly/api/", swaggerRedirect)
	e.GET("/damselfly/api/*", echoSwagger.WrapHandler)

	e.GET("/damselfly/readyz", handler.RestGetReadyz)
	e.GET("/damselfly/httpVersion", handler.RestCheckHTTPVersion)

	// for Damselfly API, set a router group which has "/damselfly" as prefix
	groupBase := e.Group("/damselfly")

	// for model API, set a router group which has "/damselfly/model" as prefix
	gModel := groupBase.Group("")
	// gModel := groupBase.Group("/model")

	gModel.GET("/model/:isTargetModel", handler.GetModels)
	gModel.GET("/model/version", handler.GetModelsVersion)

	gModel.POST("/onpremmodel", handler.CreateOnPremModel)
	gModel.GET("/onpremmodel", handler.GetOnPremModels)
	gModel.GET("/onpremmodel/:id", handler.GetOnPremModel)
	gModel.PUT("/onpremmodel/:id", handler.UpdateOnPremModel)
	gModel.DELETE("/onpremmodel/:id", handler.DeleteOnPremModel)

	gModel.POST("/cloudmodel", handler.CreateCloudModel)
	gModel.GET("/cloudmodel", handler.GetCloudModels)
	gModel.GET("/cloudmodel/:id", handler.GetCloudModel)
	gModel.PUT("/cloudmodel/:id", handler.UpdateCloudModel)
	gModel.DELETE("/cloudmodel/:id", handler.DeleteCloudModel)

	// Run Damselfly API server
	selfEndpoint := config.Damselfly.Self.Endpoint
	apidashboard := " http://" + selfEndpoint + "/damselfly/api"

	if enableAuth {
		fmt.Println(" Access to API dashboard" + " (username: " + apiUser + " / password: " + apiPass + ")")
	}
	fmt.Printf(noticeColor, apidashboard)
	fmt.Println("\n ")

	// A context for graceful shutdown (It is based on the signal package)selfEndpoint := os.Getenv("DAMSELFLY_SELF_ENDPOINT")
	// NOTE -
	// Use os.Interrupt Ctrl+C or Ctrl+Break on Windows
	// Use syscall.KILL for Kill(can't be caught or ignored) (POSIX)
	// Use syscall.SIGTERM for Termination (ANSI)
	// Use syscall.SIGINT for Terminal interrupt (ANSI)
	// Use syscall.SIGQUIT for Terminal quit (POSIX)
	gracefulShutdownContext, stop := signal.NotifyContext(context.TODO(),
		os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	// Wait graceful shutdown (and then main thread will be finished)
	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		// Block until a signal is triggered
		<-gracefulShutdownContext.Done()

		log.Info().Msg("Stopping CM-Damselfly REST API server")
		ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error when graceful shutting down CM-Damselfly API server")
			e.Logger.Panic(err)
		}
	}(&wg)

	port = fmt.Sprintf(":%s", port)

	common.SystemReady = true
	if err := e.Start(port); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Error when starting CM-Damselfly API server")
		e.Logger.Panic("Shuttig down the server: ", err)
	}

	log.Info().Msg("CM-Damselfly REST API server is started.")

	wg.Wait()
}
