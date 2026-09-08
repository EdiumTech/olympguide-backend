// @title OlympGuide API
// @version 1.0
// @contact.name Support Team
// @contact.email olympguide@mail.ru
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @BasePath /api/v1

package main

import (
	"api/handler"
	"api/middleware"
	pb "api/proto/gen"
	"api/repository"
	"api/service"
	"context"
	"github.com/gin-contrib/sessions"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"gorm.io/gorm"
	"log"

	"api/router"
	"api/utils"
)

func main() {
	cfg, err := utils.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, rdb, store, client := initConnections(cfg)
	handlers := initHandlers(db, rdb, client)
	mw := initMiddleware(db)
	utils.RegisterMetrics()

	Router := router.NewRouter(handlers, mw, store)
	Router.RegisterHealth(func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			return err
		}
		return rdb.Ping(ctx).Err()
	})
	Router.Run(cfg.ServerPort)
}

func initConnections(cfg *utils.Config) (*gorm.DB, *redis.Client, sessions.Store, *grpc.ClientConn) {
	db := utils.ConnectPostgres(cfg)
	rdb := utils.ConnectRedis(cfg)
	store := utils.ConnectSessionStore(cfg)
	client := utils.ConnectStorageService(cfg)
	return db, rdb, store, client
}

func initHandlers(db *gorm.DB, redis *redis.Client, conn *grpc.ClientConn) *handler.Handlers {
	codeRepo := repository.NewRedisCodeRepo(redis)
	userRepo := repository.NewPgUserRepo(db)
	regionRepo := repository.NewPgRegionRepo(db)
	univerRepo := repository.NewPgUniverRepo(db)
	fieldRepo := repository.NewPgFieldRepo(db)
	olympRepo := repository.NewPgOlympRepo(db)
	facultyRepo := repository.NewPgFacultyRepo(db)
	programRepo := repository.NewPgProgramRepo(db)
	diplomaRepo := repository.NewDiplomaRepo(db, redis)
	benefitRepo := repository.NewPgBenefitRepo(db)

	storageServiceClient := pb.NewStorageServiceClient(conn)

	authService := service.NewAuthService(codeRepo, userRepo, regionRepo)
	googleAuthService := service.NewExternalAuthService(userRepo, codeRepo)
	univerService := service.NewUniverService(univerRepo, regionRepo, diplomaRepo, storageServiceClient)
	fieldService := service.NewFieldService(fieldRepo)
	olympService := service.NewOlympService(olympRepo)
	metaService := service.NewMetaService(regionRepo, olympRepo, programRepo)
	userService := service.NewUserService(userRepo, regionRepo)
	facultyService := service.NewFacultyService(facultyRepo, univerRepo)
	programService := service.NewProgramService(programRepo, univerRepo, facultyRepo, fieldRepo)
	diplomaService := service.NewDiplomaService(diplomaRepo, userRepo, olympRepo)
	benefitService := service.NewBenefitService(benefitRepo, diplomaRepo)
	tokenService := service.NewTokenService()

	return &handler.Handlers{
		Auth:    handler.NewAuthHandler(authService, googleAuthService, tokenService),
		Univer:  handler.NewUniverHandler(univerService),
		Field:   handler.NewFieldHandler(fieldService),
		Olymp:   handler.NewOlympHandler(olympService),
		Meta:    handler.NewMetaHandler(metaService),
		User:    handler.NewUserHandler(userService),
		Faculty: handler.NewFacultyHandler(facultyService),
		Program: handler.NewProgramHandler(programService),
		Diploma: handler.NewDiplomaHandler(diplomaService),
		Benefit: handler.NewBenefitHandler(benefitService),
	}
}

func initMiddleware(db *gorm.DB) *middleware.Mw {
	adminRepo := repository.NewPgAdminRepo(db)
	adminService := service.NewAdminService(adminRepo)
	tokenService := service.NewTokenService()
	return middleware.NewMw(adminService, tokenService)
}
