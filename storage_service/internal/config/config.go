package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	StorageServicePort int
	MinioPort          int
	MinioHost          string
	MinioUser          string
	MinioPassword      string
	Host               string
	PublicStorageURL   string
}

func LoadConfig() (*Config, error) {
	storageServicePortStr := os.Getenv("STORAGE_SERVICE_PORT")
	storageServicePort, err := strconv.Atoi(storageServicePortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid STORAGE_SERVICE_PORT: %w", err)
	}

	minioPortStr := os.Getenv("MINIO_PORT")
	minioPort, err := strconv.Atoi(minioPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MINIO_PORT: %w", err)
	}

	publicURL := strings.TrimRight(os.Getenv("PUBLIC_STORAGE_URL"), "/")
	if publicURL != "" {
		parsed, err := url.Parse(publicURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("PUBLIC_STORAGE_URL must be an absolute http(s) URL")
		}
	} else {
		// Preserve the development Compose contract for existing installations.
		publicURL = fmt.Sprintf("%s:%d", os.Getenv("PUBLIC_HOST"), minioPort)
	}
	return &Config{
		PublicStorageURL:   publicURL,
		StorageServicePort: storageServicePort,
		MinioUser:          os.Getenv("MINIO_USER"),
		MinioPassword:      os.Getenv("MINIO_PASSWORD"),
		MinioHost:          os.Getenv("MINIO_HOST"),
		Host:               os.Getenv("PUBLIC_HOST"),
		MinioPort:          minioPort,
	}, nil
}
