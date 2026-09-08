package utils

import (
	"api/utils/constants"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"log"
	"net/http"
	"strconv"
)

func ConnectSessionStore(cfg *Config) sessions.Store {
	redisAddress := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	store, err := redis.NewStoreWithDB(
		constants.MaxSessionConn,
		"tcp",
		redisAddress,
		cfg.RedisPassword,
		strconv.Itoa(constants.RedisDBNum),
		[]byte(cfg.SessionSecret),
	)
	if err != nil {
		log.Fatalf("Could not connect to store: %v", err)
	}
	store.Options(sessions.Options{
		Path: "/", MaxAge: 14 * 24 * 60 * 60,
		HttpOnly: true, Secure: cfg.SessionSecure, SameSite: http.SameSiteLaxMode,
	})
	return store
}
