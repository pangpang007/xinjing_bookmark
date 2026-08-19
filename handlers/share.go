package handlers

import (
	"errors"
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

const maxShareUploadBytes = services.MaxShareImageBytes + 256<<10

func (h *Handler) ShareImage(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.Fail(c, utils.ErrCodeJWTInvalid, "登录已过期，请重新登录")
		return
	}

	if isLegacyJSONShare(c) {
		utils.Fail(c, utils.ErrCodeParamInvalid, "请升级后重试")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxShareUploadBytes)

	fh, err := c.FormFile("image")
	if err != nil {
		if isBodyTooLarge(err) {
			utils.Fail(c, utils.ErrCodeImageTooLarge, "图片太大，请重试")
			return
		}
		utils.Fail(c, utils.ErrCodeParamInvalid, "请上传分享图")
		return
	}
	if fh.Size > services.MaxShareImageBytes {
		utils.Fail(c, utils.ErrCodeImageTooLarge, "图片太大，请重试")
		return
	}

	src, err := fh.Open()
	if err != nil {
		utils.Fail(c, utils.ErrCodeParamInvalid, "请上传分享图")
		return
	}
	defer src.Close()

	raw, err := services.ReadAtMost(src, services.MaxShareImageBytes)
	if err != nil {
		if services.IsTooLarge(err) || isBodyTooLarge(err) {
			utils.Fail(c, utils.ErrCodeImageTooLarge, "图片太大，请重试")
			return
		}
		utils.Fail(c, utils.ErrCodeParamInvalid, "请上传分享图")
		return
	}

	jpegBytes, err := services.PrepareShareJPEG(raw)
	if err != nil {
		utils.Fail(c, utils.ErrCodeParamInvalid, "请上传分享图")
		return
	}

	if h.r2 == nil {
		utils.Fail(c, utils.ErrCodeR2UploadFail, "对象存储未配置")
		return
	}

	key := uuid.NewString() + ".jpg"
	objectKey, err := h.r2.UploadJPEG(c.Request.Context(), key, jpegBytes)
	if err != nil {
		log.Printf("[ERROR] r2 upload: %v", err)
		utils.Fail(c, utils.ErrCodeR2UploadFail, "分享图上传失败")
		return
	}

	h.archiveShare(userID, c, objectKey)
	utils.OK(c, gin.H{"image_url": h.publicShareImageURL(objectKey)})
}

func (h *Handler) archiveShare(userID int64, c *gin.Context, objectKey string) {
	lit := &models.LiteratureResponse{
		LiteratureText: strings.TrimSpace(c.PostForm("literature_text")),
		BookName:       strings.TrimSpace(c.PostForm("book_name")),
		Author:         strings.TrimSpace(c.PostForm("author")),
		Style:          models.NormalizeStyle(strings.ToLower(strings.TrimSpace(c.PostForm("style")))),
	}
	mood := strings.TrimSpace(c.PostForm("mood"))
	if lit.LiteratureText == "" && mood == "" {
		return
	}
	if mood != "" {
		h.saveHistory(userID, mood, lit, objectKey)
		return
	}
	h.attachImageURL(userID, lit, objectKey)
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

func isLegacyJSONShare(c *gin.Context) bool {
	ct := strings.ToLower(c.ContentType())
	return strings.Contains(ct, "application/json")
}

func isBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "too large") || strings.Contains(msg, "body size")
}
