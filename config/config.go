package config

import (
	"os"
	"path/filepath"
	"strings"
)

var configRootPath = "~/.mirage"

func absPath(path string) string {
	if strings.HasPrefix(path, "~") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, path[2:])
	}
	return path
}

func GetConfigRootPath() string {
	return absPath(configRootPath)
}

func SetConfigRootPath(path string) {
	configRootPath = path
}

var kubeconfig = "~/.kube/config"

func GetKubeconfig() string {
	return absPath(kubeconfig)
}

func SetKubeconfig(path string) {
	kubeconfig = path
}
