package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ModuleInfo represents module information from go list
type ModuleInfo struct {
	Path     string `json:"Path"`
	Version  string `json:"Version"`
	Main     bool   `json:"Main"`
	Dir      string `json:"Dir"`
	GoMod    string `json:"GoMod"`
	Replace  *struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
	} `json:"Replace"`
}

// GetModuleVersionCmd gets version info using go list command
func GetModuleVersionCmd(modulePath string) (*ModuleInfo, error) {
	cmd := exec.Command("go", "list", "-m", "-json", modulePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute go list: %w", err)
	}

	var moduleInfo ModuleInfo
	if err := json.Unmarshal(output, &moduleInfo); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &moduleInfo, nil
}

// GetAllModulesCmd gets all module versions using go list
func GetAllModulesCmd() ([]ModuleInfo, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute go list: %w", err)
	}

	var modules []ModuleInfo
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	
	for decoder.More() {
		var module ModuleInfo
		if err := decoder.Decode(&module); err != nil {
			return nil, fmt.Errorf("failed to decode JSON: %w", err)
		}
		modules = append(modules, module)
	}

	return modules, nil
}

// GetModuleVersionFromGoMod gets version from current directory's go.mod
func GetModuleVersionFromGoMod() (*ModuleInfo, error) {
	cmd := exec.Command("go", "list", "-m", "-json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current module info: %w", err)
	}

	var moduleInfo ModuleInfo
	if err := json.Unmarshal(output, &moduleInfo); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &moduleInfo, nil
}

func main() {
	fmt.Println("=== Current Module Info ===")
	if currentModule, err := GetModuleVersionFromGoMod(); err == nil {
		fmt.Printf("Module: %s\n", currentModule.Path)
		fmt.Printf("Version: %s\n", currentModule.Version)
		fmt.Printf("Directory: %s\n", currentModule.Dir)
		fmt.Printf("Go.mod: %s\n", currentModule.GoMod)
	} else {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Println("\n=== Specific Module Version ===")
	if moduleInfo, err := GetModuleVersionCmd("github.com/cloud-barista/cm-model"); err == nil {
		fmt.Printf("Module: %s\n", moduleInfo.Path)
		fmt.Printf("Version: %s\n", moduleInfo.Version)
		if moduleInfo.Replace != nil {
			fmt.Printf("Replaced by: %s@%s\n", moduleInfo.Replace.Path, moduleInfo.Replace.Version)
		}
	} else {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Println("\n=== All Module Versions ===")
	if modules, err := GetAllModulesCmd(); err == nil {
		for _, module := range modules {
			status := ""
			if module.Main {
				status = " (main)"
			}
			if module.Replace != nil {
				status += fmt.Sprintf(" => %s@%s", module.Replace.Path, module.Replace.Version)
			}
			fmt.Printf("%s@%s%s\n", module.Path, module.Version, status)
		}
	} else {
		fmt.Printf("Error: %v\n", err)
	}
}
