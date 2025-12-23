package utils

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func LocateCurrentDirectory() string {
	dirPath, err := os.Getwd()
	if err != nil {
		log.Fatal("failed to get current working directory", err)
	}
	rootDir := strings.Split(dirPath, "tcp_proxy")[0]
	return filepath.Join(rootDir, "tcp_proxy")
}
