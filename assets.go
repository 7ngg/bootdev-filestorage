package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func (cfg *apiConfig) getObjectURL(key string) string {
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket, cfg.s3Region, key)

	return url
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

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		filePath,
	)

	var buff bytes.Buffer
	cmd.Stdout = &buff

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe error: %v", err)
	}

	var output struct {
		Streams []struct {
			Width              int    `json:"width"`
			Height             int    `json:"height"`
			DisplayAspectRatio string `json:"display_aspect_ratio"`
		} `json:"streams"`
	}
    if err := json.Unmarshal(buff.Bytes(), &output); err != nil {
        return "", fmt.Errorf("could not parse ffprobe output: %v", err)
    }

    if len(output.Streams) == 0 {
        return "", errors.New("no video streams found")
    }

	switch output.Streams[0].DisplayAspectRatio {
	case "16:9":
		return "landscape", nil
	case "9:16":
		return "portrait", nil
	default:
		return "other", nil
	}
}
