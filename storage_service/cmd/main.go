package main

import (
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"storage_service/internal/config"
	"storage_service/internal/handler"
	"storage_service/internal/repository"
	"storage_service/internal/service"
	"storage_service/pkg/minio"
	pb "storage_service/proto/gen"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	minioClient := minio.NewMinioClient(cfg)
	logoRepo := repository.NewMinioRepository(minioClient, cfg.PublicStorageURL)
	logoService := service.NewLogoService(logoRepo)
	grpcHandler := handler.NewLogoHandler(logoService)

	runServer(grpcHandler, cfg.StorageServicePort)
}

func runServer(grpcHandler *handler.LogoHandler, port int) {
	serverAddress := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterStorageServiceServer(s, grpcHandler)

	log.Printf("gRPC server is running on port: %d", port)
	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
