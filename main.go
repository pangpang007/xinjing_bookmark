package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/soupcircle/bookjie-api/config"
	"github.com/soupcircle/bookjie-api/handlers"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("[FATAL] database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.History{}); err != nil {
		log.Fatalf("[FATAL] migrate: %v", err)
	}

	rdb := connectRedis(cfg.RedisURL)
	jwtAuth := middleware.NewJWTAuth(cfg.JWTSecret, cfg.JWTExpiry)
	deepseek := services.NewDeepSeek(cfg.DeepSeekAPIKey, cfg.DeepSeekBaseURL)
	wechat := services.NewWeChat(cfg.WeChatAppID, cfg.WeChatAppSecret, cfg.WeChatQRPage, cfg.WeChatEnvVersion, rdb)

	var imageSvc *services.ImageService
	imageSvc, err = services.NewImageService(cfg.FontPath, cfg.Timezone)
	if err != nil {
		log.Printf("[WARN] share image disabled: %v", err)
		imageSvc = nil
	}

	var r2 *services.R2
	r2, err = services.NewR2(cfg)
	if err != nil {
		log.Printf("[WARN] r2 disabled: %v", err)
		r2 = nil
	}

	h := handlers.New(cfg, db, rdb, jwtAuth, deepseek, wechat, imageSvc, r2)
	router := setupRouter(h, jwtAuth)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[INFO] bookmark api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[FATAL] shutdown: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	if rdb != nil {
		_ = rdb.Close()
	}
}

func setupRouter(h *handlers.Handler, jwtAuth *middleware.JWTAuth) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), middleware.CORS())

	mount := func(g *gin.RouterGroup) {
		g.GET("/health", h.Health)
		g.POST("/interpret", jwtAuth.Required(), h.Interpret)
		g.POST("/login", h.Login)
		g.POST("/share-image", jwtAuth.Required(), h.ShareImage)
		g.GET("/share-images/:filename", h.ShareImageFile)
		g.GET("/history", jwtAuth.Required(), h.History)
		handlers.RegisterSwagger(g)
	}

	mount(r.Group(""))
	mount(r.Group("/bookmark"))
	return r
}

func connectRedis(url string) *redis.Client {
	if url == "" {
		log.Println("[WARN] REDIS_URL empty, wechat token cache uses memory")
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("[WARN] invalid REDIS_URL: %v", err)
		return nil
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[WARN] redis unavailable: %v", err)
		return nil
	}
	return rdb
}
