package utils

import (
	"fmt"
	"google.golang.org/grpc"
	"log"
	"time"
)

func ConnectStorageService(cfg *Config) *grpc.ClientConn {
	connStr := fmt.Sprintf("%s:%d", cfg.StorageServiceHost, cfg.StorageServicePort)

	conn, err := grpc.Dial(connStr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("Failed to connect to storage service at %s: %v", connStr, err)
	}

	log.Printf("Connected to storage service at %s", connStr)
	return conn
}
