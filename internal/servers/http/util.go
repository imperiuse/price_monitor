package http

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/imperiuse/price_monitor/internal/helper"
)

// HTTPGoodResponse - http good api response.
type HTTPGoodResponse struct {
	xxxNoUnKeyLiteral [0]int // nolint
	Time              int64  `json:"time,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	NodeID            string `json:"node_id,omitempty"`
	Status            int    `json:"status,omitempty"`
	Description       string `json:"description,omitempty"`
	gin.H
}

// HTTPErrorResponse - http bad api response.
type HTTPErrorResponse struct {
	xxxNoUnKeyLiteral [0]int // nolint
	Time              int64  `json:"time,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	NodeID            string `json:"node_id,omitempty"`
	Status            int    `json:"status,omitempty"`
	Desc              string `json:"desc" example:"some human readable desc"`
	Err               string `json:"err" example:"internal server error"`
}

// SendJSON - send json (good response)
func (s *Server) SendJSON(ctx *gin.Context, status int, desc string, h gin.H) {
	ctx.JSON(status, HTTPGoodResponse{
		Time:        time.Now().UTC().Unix(),
		UUID:        helper.FromContextGetUUID(ctx),
		NodeID:      s.config.NodeID,
		Status:      status,
		Description: desc,
		H:           h,
	})
}

// SendErrorJSON - send error data in json format with status AND Abort gin ctx.
func (s *Server) SendErrorJSON(ctx *gin.Context, status int, desc string, err error) {
	var errStr = ""
	if err != nil {
		errStr = err.Error()
	}

	ctx.JSON(status, HTTPErrorResponse{
		Time:   time.Now().UTC().Unix(),
		UUID:   helper.FromContextGetUUID(ctx),
		NodeID: s.config.NodeID,
		Status: status,
		Desc:   desc,
		Err:    errStr,
	})

	ctx.Abort()
}
