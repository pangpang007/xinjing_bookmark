package handlers

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/soupcircle/bookjie-api/config"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/services"
	"github.com/soupcircle/bookjie-api/utils"
	"gorm.io/gorm"
)

type Handler struct {
	cfg      *config.Config
	db       *gorm.DB
	rdb      *redis.Client
	jwt      *middleware.JWTAuth
	deepseek *services.DeepSeek
	wechat   *services.WeChat
	image    *services.ImageService
	r2       *services.R2
	quota    *services.InterpretQuota
}

func New(
	cfg *config.Config,
	db *gorm.DB,
	rdb *redis.Client,
	jwtAuth *middleware.JWTAuth,
	deepseek *services.DeepSeek,
	wechat *services.WeChat,
	image *services.ImageService,
	r2 *services.R2,
) *Handler {
	return &Handler{
		cfg:      cfg,
		db:       db,
		rdb:      rdb,
		jwt:      jwtAuth,
		deepseek: deepseek,
		wechat:   wechat,
		image:    image,
		r2:       r2,
		quota:    services.NewInterpretQuota(rdb, cfg.Timezone, cfg.InterpretDailyLimit),
	}
}

func (h *Handler) Health(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		utils.Fail(c, utils.ErrCodeInternal, "database unavailable")
		return
	}
	utils.OK(c, gin.H{"status": "ok"})
}

func (h *Handler) saveHistory(userID int64, mood string, lit *models.LiteratureResponse, imageURL string) {
	if userID <= 0 || lit == nil || h.db == nil {
		return
	}
	row := models.History{
		UserID:         userID,
		MoodInput:      mood,
		LiteratureText: lit.LiteratureText,
		BookName:       lit.BookName,
		Author:         lit.Author,
		Style:          lit.Style,
		ImageURL:       imageURL,
	}
	if err := h.db.Create(&row).Error; err != nil {
		log.Printf("[WARN] save history: %v", err)
	}
}

func (h *Handler) attachImageURL(userID int64, lit *models.LiteratureResponse, imageURL string) {
	if userID <= 0 || lit == nil || imageURL == "" {
		return
	}
	var row models.History
	err := h.db.Where("user_id = ? AND literature_text = ? AND (image_url = '' OR image_url IS NULL)", userID, lit.LiteratureText).
		Order("id DESC").
		First(&row).Error
	if err != nil {
		h.saveHistory(userID, "", lit, imageURL)
		return
	}
	if err := h.db.Model(&row).Update("image_url", imageURL).Error; err != nil {
		log.Printf("[WARN] update history image: %v", err)
	}
}

func (h *Handler) publicShareImageURL(stored string) string {
	name := services.ShareImageFilename(stored)
	if name == "" {
		return ""
	}
	return strings.TrimRight(h.cfg.PublicBaseURL, "/") + "/share-images/" + name
}

func defaultNickname(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "心境用户"
	}
	return name
}

func resolveShareNickname(reqName, userName string) string {
	if n := strings.TrimSpace(reqName); n != "" {
		return n
	}
	return defaultNickname(userName)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
