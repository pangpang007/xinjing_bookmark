package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/services"
	"github.com/soupcircle/bookjie-api/utils"
)

type interpretRequest struct {
	Mood string `json:"mood"`
}

const recentAvoidLimit = 8

func (h *Handler) Interpret(c *gin.Context) {
	userID, loggedIn := middleware.GetUserID(c)
	if !loggedIn {
		utils.Fail(c, utils.ErrCodeJWTInvalid, "请先登录")
		return
	}

	var req interpretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, utils.ErrCodeParamInvalid, "请求参数无效")
		return
	}
	mood := strings.TrimSpace(req.Mood)
	if mood == "" {
		utils.Fail(c, utils.ErrCodeParamInvalid, "心情不能为空")
		return
	}
	if len([]rune(mood)) > 200 {
		utils.Fail(c, utils.ErrCodeParamInvalid, "心情过长")
		return
	}

	if blocked, msg := h.blockUnsafeMood(c.Request.Context(), userID, mood); blocked {
		utils.Fail(c, utils.ErrCodeContentBlocked, msg)
		return
	}

	limit := 0
	if h.cfg != nil {
		limit = h.cfg.InterpretDailyLimit
	}

	used := 0
	if limit > 0 {
		n, exceeded, err := h.quota.Incr(c.Request.Context(), userID)
		if exceeded {
			utils.Fail(c, utils.ErrCodeQuotaExceeded, fmt.Sprintf("今天已经用完 %d 次了，明天再来吧", limit))
			return
		}
		if err != nil {
			log.Printf("[WARN] interpret quota: %v", err)
		} else {
			used = n
		}
	}

	avoid := h.loadAvoidWorks(c.Request.Context(), userID)
	lit := h.deepseek.Recommend(c.Request.Context(), mood, avoid)
	h.saveHistory(userID, mood, lit, "")
	if lit != nil && used > 0 {
		lit.Quota = models.NewQuota(used, limit)
	}
	utils.OK(c, lit)
}

func (h *Handler) blockUnsafeMood(ctx context.Context, userID int64, mood string) (blocked bool, msg string) {
	if h.wechat == nil {
		return false, ""
	}

	openid := h.lookupOpenID(ctx, userID)
	if openid == "" {
		log.Printf("[WARN] msg_sec_check skipped: no openid user=%d", userID)
		return false, ""
	}

	suggest, label, err := h.wechat.MsgSecCheck(ctx, openid, mood)
	if err != nil {
		log.Printf("[WARN] msg_sec_check: %v", err)
		return false, ""
	}
	blocked, msg = interpretMoodDecision(suggest, label)
	if blocked {
		log.Printf("[INFO] msg_sec_check blocked user=%d label=%d", userID, label)
	}
	return blocked, msg
}

func interpretMoodDecision(suggest string, label int) (blocked bool, msg string) {
	if !services.MsgSecBlocked(suggest) {
		return false, ""
	}
	return true, services.MsgSecCheckUserMsg(label)
}

func (h *Handler) lookupOpenID(ctx context.Context, userID int64) string {
	if h.testOpenID != "" {
		return h.testOpenID
	}
	if h.db == nil || userID <= 0 {
		return ""
	}
	var user models.User
	if err := h.db.WithContext(ctx).Select("openid").First(&user, userID).Error; err != nil {
		log.Printf("[WARN] msg_sec_check lookup openid: %v", err)
		return ""
	}
	return strings.TrimSpace(user.OpenID)
}

func (h *Handler) loadAvoidWorks(ctx context.Context, userID int64) []services.AvoidWork {
	if h.db == nil || userID <= 0 {
		return nil
	}
	var rows []models.History
	err := h.db.WithContext(ctx).
		Select("book_name", "author", "literature_text").
		Where("user_id = ?", userID).
		Order("id DESC").
		Limit(recentAvoidLimit).
		Find(&rows).Error
	if err != nil {
		log.Printf("[WARN] load history for recommend: %v", err)
		return nil
	}
	out := make([]services.AvoidWork, 0, len(rows))
	for _, row := range rows {
		out = append(out, services.AvoidWork{
			BookName:       row.BookName,
			Author:         row.Author,
			LiteratureText: row.LiteratureText,
		})
	}
	return out
}
