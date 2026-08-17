package handlers

import (
	"errors"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/utils"
	"gorm.io/gorm"
)

type loginRequest struct {
	Code      string `json:"code"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type loginUser struct {
	ID        int64  `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, utils.ErrCodeParamInvalid, "请求参数无效")
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		utils.Fail(c, utils.ErrCodeParamInvalid, "code 不能为空")
		return
	}

	openid, err := h.wechat.Code2OpenID(c.Request.Context(), code)
	if err != nil {
		log.Printf("[WARN] wechat login: %v", err)
		utils.Fail(c, utils.ErrCodeWechatFail, "微信登录失败")
		return
	}

	nickname := defaultNickname(req.Nickname)
	avatar := strings.TrimSpace(req.AvatarURL)

	var user models.User
	err = h.db.Where("openid = ?", openid).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = models.User{
			OpenID:    openid,
			Nickname:  nickname,
			AvatarURL: avatar,
		}
		if err := h.db.Create(&user).Error; err != nil {
			log.Printf("[ERROR] create user: %v", err)
			utils.Fail(c, utils.ErrCodeInternal, "创建用户失败")
			return
		}
	} else if err != nil {
		log.Printf("[ERROR] query user: %v", err)
		utils.Fail(c, utils.ErrCodeInternal, "查询用户失败")
		return
	} else {
		updates := map[string]interface{}{}
		if req.Nickname != "" {
			updates["nickname"] = nickname
			user.Nickname = nickname
		}
		if avatar != "" {
			updates["avatar_url"] = avatar
			user.AvatarURL = avatar
		}
		if len(updates) > 0 {
			if err := h.db.Model(&user).Updates(updates).Error; err != nil {
				log.Printf("[WARN] update user profile: %v", err)
			}
		}
	}

	token, err := h.jwt.Generate(user.ID)
	if err != nil {
		utils.Fail(c, utils.ErrCodeInternal, "签发登录态失败")
		return
	}

	utils.OK(c, gin.H{
		"token": token,
		"user": loginUser{
			ID:        user.ID,
			Nickname:  user.Nickname,
			AvatarURL: user.AvatarURL,
		},
	})
}
