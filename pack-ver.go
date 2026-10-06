package main

import (
	"fmt"
	"runtime/debug"
)

// GetPackageVersion retrieves version info for a specific package
func GetPackageVersion(packagePath string) (string, bool) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}

	// Check main module
	if buildInfo.Main.Path == packagePath {
		return buildInfo.Main.Version, true
	}

	// Check dependencies
	for _, dep := range buildInfo.Deps {
		if dep.Path == packagePath {
			return dep.Version, true
		}
	}

	return "", false
}

// GetAllPackageVersions returns all package versions used in the build
func GetAllPackageVersions() map[string]string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	versions := make(map[string]string)
	
	// Add main module
	if buildInfo.Main.Path != "" {
		versions[buildInfo.Main.Path] = buildInfo.Main.Version
	}

	// Add all dependencies
	for _, dep := range buildInfo.Deps {
		versions[dep.Path] = dep.Version
	}

	return versions
}

// PrintBuildInfo displays comprehensive build information
func PrintBuildInfo() {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("Build info not available")
		return
	}

	fmt.Printf("Go Version: %s\n", buildInfo.GoVersion)
	fmt.Printf("Main Module: %s@%s\n", buildInfo.Main.Path, buildInfo.Main.Version)
	
	if len(buildInfo.Settings) > 0 {
		fmt.Println("\nBuild Settings:")
		for _, setting := range buildInfo.Settings {
			fmt.Printf("  %s: %s\n", setting.Key, setting.Value)
		}
	}

	fmt.Println("\nDependencies:")
	for _, dep := range buildInfo.Deps {
		replaced := ""
		if dep.Replace != nil {
			replaced = fmt.Sprintf(" => %s@%s", dep.Replace.Path, dep.Replace.Version)
		}
		fmt.Printf("  %s@%s%s\n", dep.Path, dep.Version, replaced)
	}
}

func main() {
	// Example usage
	fmt.Println("=== Package Version Info ===")
	
	path := "github.com/cloud-barista/cm-model"
	// Get specific package version
	if version, found := GetPackageVersion(path); found {
		fmt.Printf("%s version: %s\n", path, version)
	} else {
		fmt.Println("not found in dependencies")
	}

	fmt.Println("\n=== All Package Versions ===")
	versions := GetAllPackageVersions()
	for pkg, ver := range versions {
		fmt.Printf("%s: %s\n", pkg, ver)
	}

	fmt.Println("\n=== Complete Build Info ===")
	PrintBuildInfo()
}

