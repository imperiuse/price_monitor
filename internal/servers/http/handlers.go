package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/imperiuse/price_monitor/internal/helper"
	"github.com/imperiuse/price_monitor/internal/logger/field"
	mw "github.com/imperiuse/price_monitor/internal/servers/http/middlerware"
	"github.com/imperiuse/price_monitor/internal/servers/http/util"
	"github.com/imperiuse/price_monitor/internal/services/storage"
	"github.com/imperiuse/price_monitor/internal/services/storage/model"
)

const defaultLimit = 10000

type (
	FormGetMonitoring struct {
		ID     int64  `uri:"id" binding:"required,min=1,max=9223372036854775807"`
		Cursor uint64 `form:"cursor"  binding:"omitempty,min=0,max=18446744073709551615"`
		Limit  uint64 `form:"limit"  binding:"omitempty,min=1,max=10000"`
	}

	FormPostMonitoring struct {
		// TODO ALSO CAN BE LIKE HERE
		//FromTime  time.Time `form:"from" binding:"required" time_format:"2006-01-02T15:04:05" time_utc:"0"` // time.RFC3339
		//ToTime    time.Time `form:"to" binding:"required" time_format:"2006-01-02T15:04:05" time_utc:"0"`   // time.RFC3339

		Period    string `form:"period" binding:"required,min=2,max=10"` // 30s, 1m, 1h
		Frequency string `form:"freq" binding:"required,min=2,max=10"`   // 1s, 5s, 1m
		Currency  string `form:"cur" binding:"required,min=3,max=10"`    // btcusd
	}

	ResponsePrice struct {
		Time  time.Time `json:"time"`
		Price float64   `json:"price"`
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

// Readiness godoc
// @Summary Ready check
// @Description ready check
// @Id Ready
// @Tags Server Base
// @Accept  json
// @Produce  json
// @Success 200
// @Router /ready [get]
func Readiness(c *gin.Context) {
	c.String(http.StatusOK, "ready")
}

// GetMonitoring godoc
// @Summary Get Monitoring data
// @Description get monitoring data
// @Id GetMonitoring
// @Tags Server API
// @Param id path int true "id of road controller"
// @Param cursor query int false "cursor for cursor pagination"
// @Param limit query int false "limit for limit pagination"// todo https://uxdesign.cc/why-facebook-says-cursor-pagination-is-the-greatest-d6b98d86b6c0
// @Accept  json
// @Produce  json
// @Success 200
// @Success 202 {object} util.HTTPErrorResponse // todo https://softwareengineering.stackexchange.com/questions/316208/http-status-code-for-still-processing
// @Failure 400 {object} util.HTTPErrorResponse
// @Failure 500 {object} util.HTTPErrorResponse
// @Router /api/v1/monitoring/{id} [get]
func (s *Server) GetMonitoring(c *gin.Context) { // TODO  toooo-looong func need separate onto smallest func also probably divide onto controllers  (hex. architect)
	ctx, cf := context.WithCancel(
		helper.NewContextWithUUID(c.Copy().Request.Context(), c.GetString(mw.XRequestID)),
	) // todo probably need other settings like requestCtxTimeout
	defer cf()

	f := FormGetMonitoring{}
	if err := c.ShouldBindUri(&f); err != nil {
		s.log.Error("can not parse params (uri)", field.ID(f.ID), field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusBadRequest, "can not parse params (uri)", err)

		return
	}

	if err := c.ShouldBindQuery(&f); err != nil {
		s.log.Error("can not parse params (query)", field.ID(f.ID), field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusBadRequest, "can not parse params (query)", err)

		return
	}

	if f.Limit == 0 {
		f.Limit = defaultLimit
	}

	var m model.Monitoring
	err := s.storage.Connector().Repo(m).Get(ctx, f.ID, &m)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.log.Debug("not found monitoring obj with id", field.ID(f.ID))
			util.SendErrorJSON(c, http.StatusNotFound, "no monitoring obj with that id", nil)

			return
		}

		s.log.Error("can not get data for monitoring from db", field.ID(f.ID), field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusInternalServerError, "can not get data for monitoring from db", err)

		return
	}

	if time.Now().UTC().Before(m.ExpiredAt) {
		s.log.Debug("monitoring has not finished yet", field.Any("form", f), field.ID(f.ID))
		util.SendErrorJSON(c, http.StatusAccepted, "monitoring has not finished yet", nil)

		return
	}

	curCode, err := s.getCurrencyCodeById(ctx, m.CurrencyID)
	if err != nil {
		s.log.Debug("not found currency code by code id", field.Any("m", m), field.ID(f.ID))
		util.SendErrorJSON(c, http.StatusInternalServerError, "not found currency code by code id", nil)

		return
	}

	freq, err := time.ParseDuration(m.Frequency)
	if err != nil {
		s.log.Error("bad value for frequency from db", field.ID(f.ID), field.Any("m", m), field.Error(err))
		util.SendErrorJSON(c, http.StatusInternalServerError, "bad value for frequency from db ", err)

		return
	}

	prices := make([]model.Price, 0, f.Limit)
	// TODO Pagination (cursor, page, or other)
	err = s.storage.Connector().RepoByName(model.PriceTableNameGetterFunc(curCode)).
		Select(ctx,
			storage.
				Select("time, price").
				Where("time BETWEEN ? AND ?", m.StartedAt, m.ExpiredAt).
				OrderBy("time"),
			&prices,
		)

	if err != nil {
		s.log.Error("can not get prices data for monitoring",
			field.ID(f.ID), field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusInternalServerError, "can not get prices data for monitoring", err)

		return
	}

	// Yes I understand that this solution not perfect, and probably really not good, but without any addional info
	//for test task. I think this variant of architect is convenient now, but of course we can remove data consumption
	//from db and in the app, but we need more complicated architect
	// todo probably should work thi approach https://stackoverflow.com/questions/39334814/how-to-extract-hour-from-query-in-postgres
	prices = applyFreqFilter(prices, freq)

	// TODO optional we can delete monitoring with that ID  (auto clean table, good idea imho)
	//_, err = s.storage.Connector().Repo(m).Delete(ctx, f.ID)
	//if err != nil {
	//	s.log.Error("can not deleter monitoring", field.ID(f.ID), field.Any("form", f), field.Error(err))
	//}

	util.SendJSON(c, http.StatusOK, util.HTTPGoodResponse{
		Time:        time.Now().UTC().Unix(),
		UUID:        helper.FromContextGetUUID(ctx),
		Status:      "Ok",
		Description: "Result of monitoring (time in UTC)",
		H: gin.H{
			"MonitoringID": f.ID,
			"Prices":       convertToResponsePrices(prices),
		},
	})
}

func (s *Server) getCurrencyCodeById(ctx context.Context, id model.Identity) (string, error) {
	cur := model.Currency{}
	if err := s.storage.Connector().Repo(cur).Get(ctx, id, &cur); err != nil {
		return "", err
	}

	return cur.CurrencyCode, nil
}

func applyFreqFilter(prices []model.Price, freq time.Duration) []model.Price {
	if len(prices) == 0 {
		return prices
	}

	t := prices[0].Time
	r := append(make([]model.Price, 0, len(prices)), prices[0])

	for _, v := range prices[1:] {
		if v.Time.Before(t.Add(freq)) {
			continue
		}

		t = v.Time

		r = append(r, v)
	}

	return r
}

func convertToResponsePrices(prices []model.Price) []ResponsePrice {
	r := make([]ResponsePrice, 0, len(prices))

	for _, v := range prices {
		r = append(r, ResponsePrice{
			Time:  v.Time,
			Price: v.Price,
		})
	}

	return r
}

// PostMonitoring godoc
// @Summary Create new monitoring
// @Description create new monitoring
// @Id PostMonitoring
// @Tags Server API
// @Accept  json
// @Produce  json
// @Param cur query string true "currency code"
// @Param period path string true "limit in time like 10m"
// @Param freq path int string true "frequence like 5s"
// @Success 200 {object} util.HTTPGoodResponse
// @Failure 400 {object} util.HTTPErrorResponse
// @Failure 500 {object} util.HTTPErrorResponse
// @Router /api/v1/monitoring [post]
func (s *Server) PostMonitoring(c *gin.Context) { // TODO  toooo-looong func need separate onto smallest func  also probably divide onto controllers  (hex. architect)
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
		s.log.Error("bad value for period", field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusBadRequest, "bad value for period", err)

		return
	}

	freqDur, err := time.ParseDuration(f.Frequency)
	if err != nil {
		s.log.Error("bad value for frequncy", field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusBadRequest, "bad value for frequency", err)

		return
	}

	if f.Period == "" || f.Frequency == "" || f.Currency == "" {
		s.log.Error("bad value fin form", field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusBadRequest, "bad values in form", err)

		return
	}

	// TODO need clarify this, I add my constraints instead
	if periodDuration > time.Hour*24 || freqDur < time.Second {
		s.log.Error("period to much or freq too low", field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusBadRequest, "freqDur", err)

		return
	}

	var m model.Monitoring
	m.Frequency = f.Frequency
	m.StartedAt = time.Now().UTC()
	m.ExpiredAt = m.StartedAt.Add(periodDuration)

	m.CurrencyID, err = s.getCurrencyIdByCurrencyCode(ctx, f.Currency)
	if err != nil {
		s.log.Error("can not get data currency data from db", field.Any("form", f), field.Error(err))
		util.SendErrorJSON(c, http.StatusInternalServerError, "can not get data currency data from db", err)

		return
	}

	id, err := s.storage.Connector().Repo(m).Create(ctx, m)
	if err != nil {
		s.log.Error("can not create new monitor obj", field.Any("m", m), field.Error(err))
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

// todo refactor (optimize and cache value for currency code in hash map
func (s *Server) getCurrencyIdByCurrencyCode(ctx context.Context, currency string) (model.Identity, error) {
	currencyCode := func(s string) model.CurrencyCode {
		switch strings.ToUpper(s) {
		case model.BtcUsd:
			return model.BtcUsd
		default:
			return ""
		}
	}(currency)

	cur := model.Currency{}
	if err := s.storage.Connector().Repo(cur).FindOneBy(
		ctx,
		[]storage.Column{"id"},
		storage.Eq{"currency_code": currencyCode},
		&cur,
	); err != nil {
		return 0, err
	}

	return cur.ID, nil
}
