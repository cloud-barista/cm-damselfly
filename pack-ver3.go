// version.go - Create this in your package
package main

import (
	"fmt"
	"runtime/debug"
	// "time"
)

var (
	// These will be set at build time using -ldflags
	Version   = "dev"
	Commit    = "unknown"
	Date      = "unknown"
	GoVersion = "unknown"
)

// VersionInfo holds comprehensive version information
type VersionInfo struct {
	Version      string            `json:"version"`
	Commit       string            `json:"commit"`
	Date         string            `json:"date"`
	GoVersion    string            `json:"go_version"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// GetVersion returns version information
func GetVersion() VersionInfo {
	info := VersionInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: GoVersion,
	}

	// Try to get build info
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if info.GoVersion == "unknown" {
			info.GoVersion = buildInfo.GoVersion
		}

		// Add dependency versions
		info.Dependencies = make(map[string]string)
		for _, dep := range buildInfo.Deps {
			info.Dependencies[dep.Path] = dep.Version
		}

		// If version is still dev, try to get from build info
		if info.Version == "dev" && buildInfo.Main.Version != "(devel)" {
			info.Version = buildInfo.Main.Version
		}
	}

	return info
}

// PrintVersion prints formatted version information
func PrintVersion() {
	info := GetVersion()
	fmt.Printf("Version:    %s\n", info.Version)
	fmt.Printf("Commit:     %s\n", info.Commit)
	fmt.Printf("Date:       %s\n", info.Date)
	fmt.Printf("Go Version: %s\n", info.GoVersion)

	if len(info.Dependencies) > 0 {
		fmt.Println("\nDependencies:")
		for path, version := range info.Dependencies {
			fmt.Printf("  %s: %s\n", path, version)
		}
	}
}

// BuildVersionAwareBinary demonstrates build-time version injection
func BuildVersionAwareBinary() {
	fmt.Println(`
To build with version information, use:

go build -ldflags "-X main.Version=v1.0.0 -X main.Commit=$(git rev-parse HEAD) -X main.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.GoVersion=$(go version | awk '{print $3}')"

Or create a Makefile:

VERSION := $(shell git describe --tags --always)
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION := $(shell go version | awk '{print $$3}')

LDFLAGS := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.Date=$(DATE) -X main.GoVersion=$(GO_VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o myapp
`)
}

// GetPackageVersionWithFallback gets package version with multiple fallback methods
func GetPackageVersionWithFallback(packagePath string) (string, string, error) {
	// Method 1: Try runtime/debug
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range buildInfo.Deps {
			if dep.Path == packagePath {
				return dep.Version, "runtime/debug", nil
			}
		}
	}

	// Method 2: Try go list command (if available)
	// This would require exec.Command which might not always be available

	return "", "", fmt.Errorf("package %s not found", packagePath)
}

func main() {
	fmt.Println("=== Application Version Info ===")
	PrintVersion()

	fmt.Println("\n=== Package Version Lookup ===")
	if version, method, err := GetPackageVersionWithFallback("github.com/cloud-barista/cm-model"); err == nil {
		fmt.Printf("Package: golang.org/x/text\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Method:  %s\n", method)
	} else {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Println("\n=== Build Instructions ===")
	BuildVersionAwareBinary()
}

