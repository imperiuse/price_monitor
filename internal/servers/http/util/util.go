package util

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/imperiuse/price_monitor/internal/helper"
)

const HTTPStatusOk = "OK"

// HTTPGoodResponse - http good api response.
type HTTPGoodResponse struct {
	xxxNoUnKeyLiteral [0]int // nolint
	Time              int64  `json:"time,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	Status            string `json:"status,omitempty"`
	Description       string `json:"description,omitempty"`
	gin.H
}

// HTTPErrorResponse - http bad api response.
type HTTPErrorResponse struct {
	xxxNoUnKeyLiteral [0]int // nolint
	Time              int64  `json:"time,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	Status            int    `json:"status,omitempty"`
	Desc              string `json:"desc" example:"some human readable desc"`
	Err               string `json:"err" example:"internal server error"`
}

// SendJSON - send error data in json format with status AND Abort gin ctx.
func SendJSON(ctx *gin.Context, status int, response HTTPGoodResponse) {
	ctx.JSON(status, response)
}

// SendErrorJSON - send error data in json format with status AND Abort gin ctx.
func SendErrorJSON(ctx *gin.Context, status int, desc string, err error) {
	var errStr = ""
	if err != nil {
		errStr = err.Error()
	}

	ctx.JSON(status, HTTPErrorResponse{
		Time:   time.Now().UTC().Unix(),
		UUID:   helper.FromContextGetUUID(ctx),
		Status: status,
		Desc:   desc,
		Err:    errStr,
	})

	ctx.Abort()
}
