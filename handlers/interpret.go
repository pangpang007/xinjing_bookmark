package handlers

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/utils"
)

type interpretRequest struct {
	Mood string `json:"mood"`
}

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

	lit := h.deepseek.Recommend(c.Request.Context(), mood)
	h.saveHistory(userID, mood, lit, "")
	if lit != nil && used > 0 {
		lit.Quota = models.NewQuota(used, limit)
	}
	utils.OK(c, lit)
}
