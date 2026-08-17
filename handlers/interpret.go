package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/utils"
)

type interpretRequest struct {
	Mood string `json:"mood"`
}

func (h *Handler) Interpret(c *gin.Context) {
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

	lit := h.deepseek.Recommend(c.Request.Context(), mood)
	if userID, ok := middleware.GetUserID(c); ok {
		h.saveHistory(userID, mood, lit, "")
	}
	utils.OK(c, lit)
}
