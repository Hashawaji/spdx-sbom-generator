// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"spdx-sbom-generator/internal/helper"
	"spdx-sbom-generator/internal/models"
	"strings"
)

// Virtual env constants
const manifestSetupPy = "setup.py"
const manifestSetupCfg = "setup.cfg"
const ModuleDotVenv = ".venv"
const ModuleVenv = "venv"
const PyvenvCfg = "pyvenv.cfg"
const VirtualEnv = "VIRTUAL_ENV"

var errorWheelFileNotFound = fmt.Errorf("Wheel file not found")
var errorUnableToOpenWheelFile = fmt.Errorf("Unable to open Wheel")

func IsRequirementMeet(root bool, data string) bool {
	_modules := LoadModules(data)
	if root && len(_modules) == 1 {
		return true
	} else if !root && len(_modules) > 3 {
		return true
	}
	return false
}

func GetVenFromEnvs() (bool, string, string) {
	venvfullpath := os.Getenv(VirtualEnv)
	splitstr := strings.Split(venvfullpath, "/")
	venv := splitstr[len(splitstr)-1]
	if len(venvfullpath) > 0 {
		return true, venv, venvfullpath
	}
	return false, venv, venvfullpath
}

func HasDefaultVenv(path string) (bool, string, string) {
	modules := []string{ModuleDotVenv, ModuleVenv}
	for i := range modules {
		venvfullpath := filepath.Join(path, modules[i])
		if helper.Exists(filepath.Join(path, modules[i])) {
			return true, modules[i], venvfullpath
		}
	}
	return false, "", ""
}

func HasPyvenvCfg(path string) bool {
	return helper.Exists(filepath.Join(path, PyvenvCfg))
}

func ScanPyvenvCfg(files *string, folderpath *string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		if info.IsDir() {
			if HasPyvenvCfg(path) {
				*files = info.Name()
				p, _ := filepath.Abs(path)
				*folderpath = p

				// This is to break the walk for first enviironment found.
				// The assumption is there will be only one environment present
				return io.EOF
			}
		}
		return nil
	}
}

func SearchVenv(path string) (bool, string, string) {
	var venv string
	var venvfullpath string
	var state bool

	// if virtual env is active
	state, venv, venvfullpath = GetVenFromEnvs()
	if state {
		return true, venv, venvfullpath
	}

	state, venv, venvfullpath = HasDefaultVenv(path)
	if state {
		return state, venv, venvfullpath
	}

	err := filepath.Walk(path, ScanPyvenvCfg(&venv, &venvfullpath))
	if err == io.EOF {
		err = nil
	}

	if err == nil {
		return true, venv, venvfullpath
	}

	return false, venv, venvfullpath
}

func IsValidRootModule(path string) bool {
	modules := []string{manifestSetupCfg, manifestSetupPy}
	for i := range modules {
		if helper.Exists(filepath.Join(path, modules[i])) {
			return true
		}
	}
	return false
}

func getPackageChecksum(packagename string, packageJsonURL string, packageWheelPath string) *models.CheckSum {
	checkfortag := true

	wheeltag, err := getWheelDistributionLastTag(packageWheelPath)
	if err != nil {
		checkfortag = false
	}
	if checkfortag && len(wheeltag) == 0 {
		checkfortag = false
	}

	checksum := getPypiPackageChecksum(packagename, packageJsonURL, checkfortag, wheeltag)
	return &checksum
}

func getWheelDistributionLastTag(packageWheelPath string) (string, error) {
	if !helper.Exists(packageWheelPath) {
		return "", errorWheelFileNotFound
	}

	file, err := os.Open(packageWheelPath)
	if err != nil {
		return "", errorUnableToOpenWheelFile
	}
	defer file.Close()

	lasttag := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		res := strings.Split(scanner.Text(), ":")
		if strings.Compare(strings.ToLower(res[0]), "tag") == 0 {
			lasttag = strings.TrimSpace(res[1])
		}
	}

	return lasttag, nil
}
