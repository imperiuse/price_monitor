package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imperiuse/price_monitor/internal/helper"
	mw "github.com/imperiuse/price_monitor/internal/servers/http/middlerware"
	"github.com/imperiuse/price_monitor/internal/servers/http/util"
	"github.com/imperiuse/price_monitor/internal/services/storage"
	"github.com/imperiuse/price_monitor/internal/services/storage/model"
)

type (
	FormGetMonitoring struct {
		ID string `uri:"id" binding:"required,min=1,max=9_223_372_036_854_775_807"`
	}

	FormPostMonitoring struct {
		// TODO ALSO CAN BE LIKE HERE
		//FromTime  time.Time `form:"from" binding:"required" time_format:"2006-01-02T15:04:05" time_utc:"0"` // time.RFC3339
		//ToTime    time.Time `form:"to" binding:"required" time_format:"2006-01-02T15:04:05" time_utc:"0"`   // time.RFC3339

		Period    string `form:"period" binding:"required,min=2,max=10"` // 30s, 1m, 1h
		Frequency string `form:"freq" binding:"required,min=2,max=10"`   // 1s, 5s, 1m
		Currency  string `form:"cur" binding:"required,min=3,max=10"`    // time.RFC3339
	}
)

// Health godoc
// @Summary Health check
// @Description health check
// @Id Health
// @Tags Server Base
// @Accept  json
// @Produce  json
// @Success 200
// @Router /health [get]
func Health(c *gin.Context) {
	c.String(http.StatusOK, "")
}

func (s *Server) GetMonitoring(c *gin.Context) {
	ctx, cf := context.WithCancel(
		helper.NewContextWithUUID(c.Copy().Request.Context(), c.GetString(mw.XRequestID)),
	) // todo probably need other settings like requestCtxTimeout
	defer cf()

	f := FormGetMonitoring{}
	if c.BindUri(f) != nil {
		return
	}

	var m model.Monitoring
	err := s.storage.Connector().Repo(m).Get(ctx, f.ID, &m)
	if err != nil {
		util.SendErrorJSON(c, http.StatusInternalServerError, "can not get data from db", err)

		return
	}
}

func (s *Server) PostMonitoring(c *gin.Context) {
	ctx, cf := context.WithCancel(
		helper.NewContextWithUUID(c.Copy().Request.Context(), c.GetString(mw.XRequestID)),
	) // todo probably need other settings like requestCtxTimeout
	defer cf()

	var f FormPostMonitoring

	if c.Bind(&f) != nil {
		return
	}

	periodDuration, err := time.ParseDuration(f.Period)
	if err != nil {
		util.SendErrorJSON(c, http.StatusBadRequest, "bad value for period", err)

		return
	}

	_, err = time.ParseDuration(f.Frequency)
	if err != nil {
		util.SendErrorJSON(c, http.StatusBadRequest, "bad value for frequency", err)

		return
	}

	var m model.Monitoring
	m.Frequency = f.Frequency
	m.StartedAt = time.Now().UTC()
	m.ExpiredAt = m.StartedAt.Add(periodDuration)

	// todo refactor
	var currencyID int64 = 1
	{
		currencyCode := func(s string) model.CurrencyCode {
			if strings.ToUpper(s) == model.BtcUsd {
				return model.BtcUsd
			}

			return ""
		}(f.Currency)

		cur := model.Currency{}
		if err := s.storage.Connector().Repo(cur).FindBy(
			ctx,
			[]storage.Column{"id"},
			storage.Eq{"currency_code": currencyCode},
			&cur,
		); err != nil {
			util.SendErrorJSON(c, http.StatusInternalServerError, "can not get data currency data from db", err)

			return
		}

		currencyID = cur.ID
	}

	m.CurrencyID = currencyID

	id, err := s.storage.Connector().Repo(m).Create(ctx, m)
	if err != nil {
		util.SendErrorJSON(c, http.StatusInternalServerError, "can not create new monitor obj", err)

		return
	}

	util.SendJSON(c, http.StatusOK, util.HTTPGoodResponse{
		Time:        time.Now().UTC().Unix(),
		UUID:        helper.FromContextGetUUID(ctx),
		Status:      "Ok",
		Description: "Successfully created new monitoring",
		H: gin.H{
			"MonitoringID": id,
		},
	})

}
