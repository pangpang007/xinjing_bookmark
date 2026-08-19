package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/utils"
)

func (h *Handler) History(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.Fail(c, utils.ErrCodeJWTInvalid, "请先登录")
		return
	}

	page := queryInt(c, "page", 1)
	pageSize := queryInt(c, "page_size", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	var total int64
	if err := h.db.Model(&models.History{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		utils.Fail(c, utils.ErrCodeInternal, "查询历史失败")
		return
	}

	var list []models.History
	offset := (page - 1) * pageSize
	if err := h.db.Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		utils.Fail(c, utils.ErrCodeInternal, "查询历史失败")
		return
	}
	if list == nil {
		list = []models.History{}
	}
	for i := range list {
		list[i].ImageURL = h.publicShareImageURL(list[i].ImageURL)
	}

	utils.OK(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
