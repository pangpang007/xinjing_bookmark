package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/services"
	"github.com/soupcircle/bookjie-api/utils"
)

type shareRequest struct {
	LiteratureText string `json:"literature_text"`
	BookName       string `json:"book_name"`
	Author         string `json:"author"`
	Style          string `json:"style"`
	Mood           string `json:"mood"`
	Nickname       string `json:"nickname"`
	AvatarURL      string `json:"avatar_url"`
	AvatarBase64   string `json:"avatar_base64"`
}

func (h *Handler) ShareImage(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.Fail(c, utils.ErrCodeJWTInvalid, "请先登录")
		return
	}
	if h.image == nil {
		utils.Fail(c, utils.ErrCodeImageGenFail, "字体未就绪，无法生成分享图")
		return
	}
	if h.r2 == nil {
		utils.Fail(c, utils.ErrCodeR2UploadFail, "对象存储未配置")
		return
	}

	var req shareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, utils.ErrCodeParamInvalid, "请求参数无效")
		return
	}
	req.LiteratureText = strings.TrimSpace(req.LiteratureText)
	req.BookName = strings.TrimSpace(req.BookName)
	req.Author = strings.TrimSpace(req.Author)
	req.Style = models.NormalizeStyle(strings.ToLower(strings.TrimSpace(req.Style)))
	if req.LiteratureText == "" || req.BookName == "" || req.Author == "" {
		utils.Fail(c, utils.ErrCodeParamInvalid, "文学内容不完整")
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		utils.Fail(c, utils.ErrCodeJWTInvalid, "用户不存在")
		return
	}

	var qr []byte
	if png, err := h.wechat.MiniProgramCode(c.Request.Context(), fmt.Sprintf("u=%d", user.ID)); err != nil {
		log.Printf("[WARN] wxacode: %v", err)
	} else {
		qr = png
	}

	jpegBytes, err := h.image.Generate(services.ShareImageInput{
		LiteratureText: req.LiteratureText,
		BookName:       req.BookName,
		Author:         req.Author,
		Style:          req.Style,
		Nickname:       resolveShareNickname(req.Nickname, user.Nickname),
		AvatarURL:      firstNonEmpty(req.AvatarURL, user.AvatarURL),
		AvatarBase64:   req.AvatarBase64,
		QRCodePNG:      qr,
	})
	if err != nil {
		log.Printf("[ERROR] generate share image: %v", err)
		utils.Fail(c, utils.ErrCodeImageGenFail, "分享图生成失败")
		return
	}

	key := uuid.NewString() + ".jpg"
	objectKey, err := h.r2.UploadJPEG(c.Request.Context(), key, jpegBytes)
	if err != nil {
		log.Printf("[ERROR] r2 upload: %v", err)
		utils.Fail(c, utils.ErrCodeR2UploadFail, "分享图上传失败")
		return
	}

	imageURL := h.publicShareImageURL(objectKey)
	lit := &models.LiteratureResponse{
		LiteratureText: req.LiteratureText,
		BookName:       req.BookName,
		Author:         req.Author,
		Style:          req.Style,
	}
	if req.Mood != "" {
		h.saveHistory(userID, strings.TrimSpace(req.Mood), lit, objectKey)
	} else {
		h.attachImageURL(userID, lit, objectKey)
	}

	utils.OK(c, gin.H{"image_url": imageURL})
}

func (h *Handler) ShareImageFile(c *gin.Context) {
	name := services.ShareImageFilename(c.Param("filename"))
	if name == "" || h.r2 == nil {
		c.Status(http.StatusNotFound)
		return
	}
	body, contentType, size, err := h.r2.GetJPEG(c.Request.Context(), name)
	if err != nil {
		log.Printf("[WARN] share image fetch: %v", err)
		c.Status(http.StatusNotFound)
		return
	}
	defer body.Close()
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, size, contentType, body, nil)
}
