package consul

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"

	"github.com/imperiuse/price_monitor/internal/helper"
	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
)

var (
	ErrNilKV = errors.New("nil kv pair struct")
)

// RegistrableService - interface which need to impl for Consul register
type RegistrableService interface {
	GetConsulServiceRegistration(Config) *Service
}

type (
	Service       = api.AgentServiceRegistration
	Services      = []*Service
	ServiceCheck  = api.AgentServiceCheck
	ServiceChecks = api.AgentServiceChecks
	ApiPairs      = api.KVPairs
)

//go:generate moq -out ../mocks/mock_consul.go -pkg mocks . ApiConsulClientI
type (
	// Config - cfg for Consul
	Config struct {
		Address     string
		Interval    time.Duration
		Timeout     time.Duration
		Tags        []string
		DNS         []string
		SessionTTL  string `yaml:"sessionTTL"`
		WaitTimeout string `yaml:"waitTimeout"`

		sessionTTL time.Duration
	}

	// ApiConsulClientI - for mocks
	ApiConsulClientI interface {
		Agent() *api.Agent
		Health() *api.Health
		KV() *api.KV
		Session() *api.Session
	}

	// Client - custom consul client based on hashicorp client
	Client struct {
		config            Config
		log               *logger.Logger
		client            ApiConsulClientI
		sessionID         string
		hostIP            string
		timeLastLeaderAck time.Time // время последнего подтверждения лидерства
	}
)

// New - return new custom Consul client
func New(config Config, log *logger.Logger) (*Client, error) {
	c := &Client{
		config: config,
		log:    log,
	}

	var err error
	c.config.sessionTTL, err = time.ParseDuration(c.config.SessionTTL)
	if err != nil {
		return nil, fmt.Errorf("time.ParseDuration(c.config.SessionTTL): %w", err)
	}

	client, err := api.NewClient(&api.Config{
		Address: config.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating consul client: %w", err)
	}
	c.client = client
	c.timeLastLeaderAck = time.Now().UTC() // default init

	return c, nil
}

// Addresses - return list of address
func (c *Client) Addresses(dc string, serviceName string, serviceTags []string) ([]string, error) {
	entries, _, err := c.client.Health().ServiceMultipleTags(serviceName, serviceTags, true, &api.QueryOptions{
		Datacenter: dc,
	})
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(entries))
	for _, entry := range entries {
		host := entry.Service.Address
		if host == "" {
			host = entry.Node.Address
		}
		addrs = append(addrs, host+":"+strconv.Itoa(entry.Service.Port))
	}

	return addrs, nil
}

// Register - register service in Consul
func (c *Client) Register(logger *logger.Logger, services ...RegistrableService) error {
	for _, service := range services {
		consulServiceStruct := service.GetConsulServiceRegistration(c.config)
		logger.Info("[Consul] registering in consul", zap.Reflect("consul_service", consulServiceStruct))
		err := c.client.Agent().ServiceRegister(consulServiceStruct)
		if err != nil {
			logger.Error("[Consul] error registering service in consul",
				field.String("ServiceName", consulServiceStruct.Name), zap.Error(err))
			return fmt.Errorf("error registering service '%s' in consul: %w", consulServiceStruct.Name, err)
		}
	}
	return nil
}

// Deregister - deregister service in consul
func (c *Client) Deregister(logger *logger.Logger, services ...RegistrableService) {
	err := c.DestroySession()
	if err != nil {
		logger.Error("[Consul] DestroySession", zap.Error(err))
	}
	for _, service := range services {
		consulServiceStruct := service.GetConsulServiceRegistration(c.config)
		logger.Info("[Consul] Deregistering in consul", zap.Reflect("consul_service", service))
		err = c.client.Agent().ServiceDeregister(consulServiceStruct.ID)
		if err != nil {
			logger.Error("[Consul] deregistering in consul error",
				zap.Reflect("consul_service", service), zap.Error(err))
		}
	}
}

// KVPut - put key value into Consul
func (c *Client) KVPut(key string, value []byte) error {
	_, err := c.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: value,
	}, nil)

	return err
}

// KVGet - get key value from Consul
func (c *Client) KVGet(key string) ([]byte, error) {
	pair, _, err := c.client.KV().Get(key, nil)
	if err != nil {
		return nil, err
	}

	if pair == nil {
		return nil, nil
	}

	return pair.Value, nil
}

// CreateSession - create session in Consul
func (c *Client) CreateSession() (string, error) {
	sessionConf := &api.SessionEntry{
		TTL:      c.config.SessionTTL,
		Behavior: "delete",
	}

	sessionID, _, err := c.client.Session().Create(sessionConf, nil)
	if err != nil {
		return "", err
	}
	c.sessionID = sessionID
	c.hostIP = helper.GetOutboundIP(c.config.DNS...).String()

	return sessionID, nil
}

// AcquireSessionWithKey - acquire session with key in Consul
func (c *Client) AcquireSessionWithKey(key string) (bool, error) {
	KVpair := &api.KVPair{
		Key:     key,
		Value:   []byte(generateSessionStoreValue(c.sessionID, c.hostIP)),
		Session: c.sessionID,
	}

	acquired, _, err := c.client.KV().Acquire(KVpair, nil)
	return acquired, err
}

func generateSessionStoreValue(sessionID string, hostIP string) string {
	return fmt.Sprintf("%s_%s", sessionID, hostIP)
}

// IsMySessionIDInKey - check is my session in Consul KV
func (c *Client) IsMySessionIDInKey(key string) (bool, error) {
	kvPair, _, err := c.client.KV().Get(key, nil)
	if err != nil {
		return false, err
	}
	if kvPair == nil {
		return false, ErrNilKV
	}
	return string(kvPair.Value) == generateSessionStoreValue(c.sessionID, c.hostIP), nil
}

// DestroySession - destroy session
func (c *Client) DestroySession() error {
	_, err := c.client.Session().Destroy(c.sessionID, nil)
	if err != nil {
		return fmt.Errorf("error cannot delete session %s: %w", c.sessionID, err)
	}

	return nil
}

// RenewSession - renew session
func (c *Client) RenewSession() error {
	_, _, err := c.client.Session().Renew(c.sessionID, nil)
	return err
}

// RenewSessionPeriodic - renew session periodic
func (c *Client) RenewSessionPeriodic(doneChan <-chan struct{}) error {
	err := c.client.Session().RenewPeriodic(c.config.SessionTTL, c.sessionID, nil, doneChan)
	if err != nil {
		return err
	}
	return nil
}

// CheckLeadership - check leadership (also check not found any leader state)
func (c *Client) CheckLeadership(oldIsLeader bool, leaderKey string) (isLeader bool, noLeader bool) {
	isLeader, err := c.IsMySessionIDInKey(leaderKey)
	if errors.Is(err, ErrNilKV) { // в KV консула нет сведений о ключе вообще
		c.log.Warn("[Consul] no leader")

		return false, true
	} else if err != nil { // нет связи, упал консул, другая ошибка
		c.log.Error("[Consul] error while IsMySessionIDInKey()", zap.Error(err))

		// Если обрыв связи, недоступен консул и уже прошло больше времени чем живет ключ TTL сессии,
		// то считаем что мы перестали быть мастером, считаем что наверно кто то другой стал(станет мастером)
		if time.Since(c.timeLastLeaderAck) > c.config.sessionTTL {
			return false, false
		}

		return oldIsLeader, false
	}

	// happy path, мы узнали есть мы или нет
	c.timeLastLeaderAck = time.Now().UTC()
	return isLeader, false
}

// TryBecomeLeader - try to become leader with leader key
func (c *Client) TryBecomeLeader(leaderKey string) (bool, error) {
	sID, err := c.CreateSession()
	if err != nil {
		c.log.Error("[Consul] CreateSession()", zap.Error(err))
		return false, fmt.Errorf("[Consul] create session problem: %w", err)
	}
	c.log.Info("[Consul] create new session", zap.String("sessionID", sID))

	res, err := c.AcquireSessionWithKey(leaderKey)
	if !res {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("[Consul] acquire session with key problem: %w", err)
	}

	c.log.Info("[Consul] Successfully AcquireSessionWithKey")
	return true, nil
}
