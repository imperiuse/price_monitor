package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/imperiuse/price_monitor/internal/consul"
	"github.com/imperiuse/price_monitor/internal/env"
	"github.com/imperiuse/price_monitor/internal/helper"
	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
	mw "github.com/imperiuse/price_monitor/internal/servers/http/middlerware"
	"github.com/imperiuse/price_monitor/internal/services/storage"
	"go.uber.org/zap"
)

type (
	// Config - config struct.
	Config struct {
		Name        string
		NodeID      string
		Address     string
		DomainName  string `yaml:"domainName"`
		AllowOrigin string `yaml:"allowOrigin"`

		Timeouts Timeouts `yaml:"timeouts"`
	}

	// Timeouts - timeouts struct.
	Timeouts struct {
		ReadTimeout string `yaml:"readTimeout"`
		readTimeout time.Duration

		WriteTimeout string `yaml:"writeTimeout"`
		writeTimeout time.Duration
	}

	// Server - server struct
	Server struct {
		config    Config
		log       *zap.Logger
		server    *http.Server
		ginEngine *gin.Engine
		storage   storage.Storage
	}
)

// HandlerFunc - gin handle func.
type HandlerFunc = func(*gin.Context)

const (
	serviceName = "http"

	apiPath = "/api"

	apiPathVersion1 = apiPath + "/v1"

	apiPathVersion = apiPathVersion1
)

// New - create new http server.
func New(
	ev env.Var,
	config Config,
	logger *logger.Logger,
	storage storage.Storage,
) (
	*Server,
	error,
) {
	if ev == env.Prod {
		gin.SetMode(gin.ReleaseMode)
	}

	e := gin.New()
	e.ForwardedByClientIP = true
	e.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}

	var err error

	config.Timeouts.readTimeout, err = time.ParseDuration(config.Timeouts.ReadTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse servers.http.config.Timeouts.ReadTimeout: %w", err)
	}

	config.Timeouts.writeTimeout, err = time.ParseDuration(config.Timeouts.WriteTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse servers.http.config.Timeouts.WriteTimeout: %w", err)
	}

	if _, _, err = net.SplitHostPort(config.Address); err != nil {
		return nil, err
	}

	s := &Server{
		config: config,
		log:    logger.With(zap.String("service", "gin/http")),
		server: &http.Server{
			Addr:           config.Address,
			Handler:        e,
			ReadTimeout:    config.Timeouts.readTimeout,
			WriteTimeout:   config.Timeouts.writeTimeout,
			MaxHeaderBytes: 1 << 20,
		},
		ginEngine: e,
		storage:   storage,
	}

	s.log.Info("starting create routes for gin s")

	// Server's check handlers
	// todo metrics middleware -> mw.MetricsRPC("health", Health))
	e.GET("/health", s.Health)
	e.GET("/ready", s.Readiness)

	// Add a ginzap middleware, which:
	//   - Logs all requests, like a combined access and error log.
	//   - Logs to stdout.
	//   - RFC3339 with UTC time format.
	e.Use(mw.Ginzap(logger, time.RFC3339, true))

	// Logs all panic to error log
	//   - stack means whether output the stack info.
	e.Use(mw.RecoveryWithZap(logger, true))

	// https://stackoverflow.com/questions/29418478/go-gin-framework-cors
	e.Use(mw.CORSMiddleware(config.AllowOrigin))

	e.Use(mw.UUIDMiddleware())

	// TODO OTel (in real world)

	if ev == env.Dev {
		// TODO profiler and add statsviz  -> (in real world)
		// TODO Swagger Docs -> (in real world)

	}

	apiVer := e.Group(apiPathVersion)

	monitroing := apiVer.Group("/monitoring")

	monitroing.GET(":id", s.GetMonitoring)
	monitroing.POST("", s.PostMonitoring)

	return s, nil
}

// Run -run http server.
func (s *Server) Run() {
	go func() {
		s.log.Info("starting http server", field.String("address", s.config.Address))

		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Fatal("error while starting http server", field.Error(err))
		}
	}()
}

// Stop - stop server.
func (s *Server) Stop(ctx context.Context) {
	if err := s.server.Shutdown(ctx); err != nil {
		s.log.Error("error while shutdown http [gin] server", zap.Error(err))
	}

	s.log.Info("http [gin] server has been shutdown")
}

// GetConsulServiceRegistration - GetConsulServiceRegistration.
func (s *Server) GetConsulServiceRegistration(cc consul.Config) *consul.Service {
	httpHost, httpPort, _ := net.SplitHostPort(s.config.Address)

	port, _ := strconv.Atoi(httpPort)

	return &consul.Service{
		ID:      fmt.Sprintf("%s_%s_%s_%s", s.config.Name, httpHost, httpPort, strings.Join(cc.Tags, "_")),
		Name:    s.config.Name,
		Address: httpHost,
		Port:    port,
		Tags:    cc.Tags,
		Check: &consul.ServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%d/health", helper.GetOutboundIP(cc.DNS...), port),
			Interval: cc.Interval.String(),
			Timeout:  cc.Timeout.String(),
		},
	}
}
