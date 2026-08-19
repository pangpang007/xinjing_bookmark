package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/services"
	"github.com/soupcircle/bookjie-api/utils"
)

const publicWxaCodeName = "share.png"

func (h *Handler) WxaCode(c *gin.Context) {
	if _, ok := middleware.GetUserID(c); !ok {
		utils.Fail(c, utils.ErrCodeJWTInvalid, "登录已过期，请重新登录")
		return
	}
	if h.r2 == nil {
		utils.Fail(c, utils.ErrCodeR2UploadFail, "对象存储未配置")
		return
	}

	key := h.wxaCodeObjectKey()
	h.wxaMu.Lock()
	defer h.wxaMu.Unlock()

	exists, err := h.r2.Exists(c.Request.Context(), key)
	if err != nil {
		log.Printf("[WARN] wxacode head: %v", err)
	}
	if !exists {
		png, err := h.wechat.ShareWxaCode(c.Request.Context())
		if err != nil {
			log.Printf("[WARN] wxacode wechat: %v", err)
			utils.Fail(c, utils.ErrCodeWxaCodeFail, "小程序码生成失败")
			return
		}
		if _, err := h.r2.Put(c.Request.Context(), key, "image/png", png); err != nil {
			log.Printf("[ERROR] wxacode upload: %v", err)
			utils.Fail(c, utils.ErrCodeR2UploadFail, "小程序码保存失败")
			return
		}
	}

	utils.OK(c, gin.H{"wxacode_url": h.publicWxaCodeURL()})
}

func (h *Handler) WxaCodeFile(c *gin.Context) {
	if c.Param("filename") != publicWxaCodeName || h.r2 == nil {
		c.Status(http.StatusNotFound)
		return
	}
	body, contentType, size, err := h.r2.Get(c.Request.Context(), h.wxaCodeObjectKey())
	if err != nil {
		log.Printf("[WARN] wxacode fetch: %v", err)
		c.Status(http.StatusNotFound)
		return
	}
	defer body.Close()
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "image/png"
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, size, contentType, body, nil)
}

func (h *Handler) wxaCodeObjectKey() string {
	env := "release"
	if h.wechat != nil {
		env = h.wechat.EnvVersion()
	} else if h.cfg != nil && strings.TrimSpace(h.cfg.WeChatEnvVersion) != "" {
		env = h.cfg.WeChatEnvVersion
	}
	return services.WxaCodeObjectKey(env)
}

func (h *Handler) publicWxaCodeURL() string {
	base := ""
	if h.cfg != nil {
		base = strings.TrimRight(h.cfg.PublicBaseURL, "/")
	}
	return base + "/wxacode/" + publicWxaCodeName
}
