package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: ErrCodeSuccess, Data: data, Msg: "success"})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Body{Code: code, Data: nil, Msg: msg})
}
