package util

import (
	"os"
	"path/filepath"
)

func GetAbsPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		if path[0:2] == "~/" {
			path = path[2:]
			homeDir, err := os.UserHomeDir()
			if err != nil {
                return path, err
			}
			path = filepath.Join(homeDir, path)
		}
	}
    return filepath.Abs(path)
}
