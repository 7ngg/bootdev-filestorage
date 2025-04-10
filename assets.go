package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mt string) string {
	ext := mediaTypeToExt(mt)
    
    bytes := make([]byte, 32)
    rand.Read(bytes)
    name := base64.RawURLEncoding.EncodeToString(bytes)

    return name + ext
}

func (cfg *apiConfig) getAssetDiskPath(ap string) string {
	return filepath.Join(cfg.assetsRoot, ap)
}

func (cfg *apiConfig) getAssetUrl(ap string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, ap)
}

func mediaTypeToExt(mt string) string {
	parts := strings.Split(mt, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}
