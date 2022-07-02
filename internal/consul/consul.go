//nolint golint
package consul

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	jsoniter "github.com/json-iterator/go"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"

	"github.com/imperiuse/price_monitor/internal/helper"
	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
)

var (
	ErrNilKV                       = errors.New("nil kv pair struct")
	ErrEmptyKeyPrefix              = errors.New("consul key prefix could not be empty")
	ErrEmptyHostName               = errors.New("empty host name")
	ErrInvalidIP                   = errors.New("invalid IP")
	errTemplateChildBothDataAndDir = "child is both a data item and dir: %s"
)

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
	Config struct {
		Address             string
		Interval            time.Duration
		Timeout             time.Duration
		Tags                []string
		DNS                 []string
		SessionTTL          string `yaml:"sessionTTL"`
		PeriodicScanTimeout string `yaml:"periodicScanTimeout"`
		WaitTimeout         string `yaml:"waitTimeout"`

		sessionTTL time.Duration
	}

	ApiConsulClientI interface {
		Agent() *api.Agent
		Health() *api.Health
		KV() *api.KV
		Session() *api.Session
	}

	Client struct {
		config              Config
		log                 *logger.Logger
		client              ApiConsulClientI
		sessionID           string
		hostIP              string
		scan                []*ScannerKV
		scanWaitTimeout     time.Duration
		scanPeriodicTimeout time.Duration
		timeLastLeaderAck   time.Time // время последнего подтверждения лидерства
	}

	ScannerKV struct {
		KeyPrefix string
		Update    chan ApiPairs // OUT send new value of consul key
		ErrCh     chan error    // OUT send errors which happens when scanner will be working
	}
)

func NewMock(config Config, log *logger.Logger, client ApiConsulClientI) *Client {
	return &Client{
		config: config,
		log:    log,
		client: client,
	}
}

func New(config Config, log *logger.Logger) (*Client, error) {
	c := &Client{
		config: config,
		log:    log,
	}

	d, err := time.ParseDuration(config.WaitTimeout)
	if err != nil {
		return nil, fmt.Errorf("time.ParseDuration(config.WaitTimeout): %w", err)
	}
	c.scanWaitTimeout = d

	d, err = time.ParseDuration(config.PeriodicScanTimeout)
	if err != nil {
		return nil, fmt.Errorf("time.ParseDuration(config.PeriodicScanTimeout): %w", err)
	}
	c.scanPeriodicTimeout = d

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

func (c *Client) KVPut(key string, value []byte) error {
	_, err := c.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: value,
	}, nil)

	return err
}

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

func (c *Client) GetStorageLeaderHostName(key string) (string, error) {
	hostName, err := c.KVGet(key)

	if err != nil {
		return "", err
	}

	if len(hostName) == 0 {
		return "", ErrEmptyHostName
	}

	return CheckIsIPValid(string(hostName))
}

func (c *Client) GetStorageSlaveHostName(key string) (string, error) {
	hostNames, _, err := c.client.KV().List(key, nil)

	if err != nil {
		return "", err
	}

	if len(hostNames) == 0 {
		return "", ErrEmptyHostName
	}

	type MemberStruct struct {
		ConnUrl      string `yaml:"conn_url"`
		ApiUrl       string `yaml:"api_url"`
		State        string `yaml:"state"`
		Role         string `yaml:"role"`
		Version      string `yaml:"version"`
		XLogLocation string `yaml:"xlog_location"`
		Timeline     int    `yaml:"timeline"`
	}

	const masterRole = "master"

	var member MemberStruct
	for _, v := range hostNames {
		if err = jsoniter.Unmarshal(v.Value, &member); err != nil {
			c.log.Error("[GetStorageSlaveHostName] problem Unmarshal data to MemberStruct")

			continue
		}

		if member.Role == masterRole {
			continue
		}

		ss := strings.Split(v.Key, key+"/") // try get last part of name this is IP addr of slave
		if len(ss) != 2 {
			continue
		}

		hostName := ss[1]

		if hostName, err = CheckIsIPValid(hostName); hostName != "" && err == nil {
			return hostName, nil
		}
	}

	return "", ErrEmptyHostName
}

// CheckIsIPValid - check that IP is valid.
func CheckIsIPValid(s string) (string, error) {
	// special hack only for stage docker-compose environment
	if s == "core-dev-postgres" {
		return s, nil
	}

	ip, _, err := net.SplitHostPort(s)
	if err == nil {
		return ip, nil
	}

	ip2 := net.ParseIP(s)
	if ip2 == nil {
		return "", ErrInvalidIP
	}

	return ip2.String(), nil
}

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

func (c *Client) DestroySession() error {
	_, err := c.client.Session().Destroy(c.sessionID, nil)
	if err != nil {
		return fmt.Errorf("error cannot delete session %s: %w", c.sessionID, err)
	}

	return nil
}

func (c *Client) RenewSession() error {
	_, _, err := c.client.Session().Renew(c.sessionID, nil)
	return err
}

func (c *Client) RenewSessionPeriodic(doneChan <-chan struct{}) error {
	err := c.client.Session().RenewPeriodic(c.config.SessionTTL, c.sessionID, nil, doneChan)
	if err != nil {
		return err
	}
	return nil
}

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

func (c *Client) StartPeriodicScan(ctx context.Context, keyPrefix string) (*ScannerKV, error) {
	scanner, err := c.newScannerKV(ctx, keyPrefix)
	if err != nil {
		return scanner, err
	}

	c.scan = append(c.scan, scanner)
	return scanner, nil
}

func (c *Client) newScannerKV(ctx context.Context, keyPrefix string) (*ScannerKV, error) {
	if keyPrefix == "" {
		return nil, ErrEmptyKeyPrefix
	}

	if keyPrefix[len(keyPrefix)-1] != '/' {
		keyPrefix += "/"
	}

	scanner := &ScannerKV{
		KeyPrefix: keyPrefix,
		Update:    make(chan api.KVPairs),
		ErrCh:     make(chan error),
	}

	go func() {
		var waitIndex uint64
		for {
			// Setup our variables and query options for the query
			var (
				pairs api.KVPairs
				meta  *api.QueryMeta
			)

			queryOpts := &api.QueryOptions{
				WaitIndex: waitIndex,
				WaitTime:  c.scanWaitTimeout,
			}

			// Perform a query with exponential backoff to get our pairs
			err := backoff.Retry(func() error {
				select {
				case <-ctx.Done():
					return nil
				default:
				}

				// Query
				var err error
				pairs, meta, err = c.client.KV().List(keyPrefix, queryOpts)

				if err != nil {
					scanner.ErrCh <- err
				}

				return err
			}, newBackOff())
			if err != nil {
				// These get sent by list
				continue
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			// If we have the same index, then we didn't find any new values.
			if meta.LastIndex == waitIndex {
				continue
			}

			// Update our wait index
			waitIndex = meta.LastIndex

			// Send the pairs
			scanner.Update <- pairs
		}
	}()

	return scanner, nil
}

func (s *ScannerKV) Decode(pairs api.KVPairs, target any) (any, error) {
	raw := make(map[string]any)
	for _, p := range pairs {
		// Trim the prefix off our key first
		key := strings.TrimPrefix(p.Key, s.KeyPrefix)

		// Determine what map we're writing the value to. We split by '/'
		// to determine any sub-maps that need to be created.
		m := raw
		children := strings.Split(key, "/")
		if len(children) > 0 {
			key = children[len(children)-1]
			children = children[:len(children)-1]
			for _, child := range children {
				if m[child] == nil {
					m[child] = make(map[string]any)
				}

				subm, ok := m[child].(map[string]any)
				if !ok {
					return nil, fmt.Errorf(errTemplateChildBothDataAndDir, child)
				}

				m = subm
			}
		}

		m[key] = string(p.Value)
	}

	// First copy our initial value
	res, err := copystructure.Copy(target)
	if err != nil {
		return res, err
	}

	// Now decode into it
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           res,
		WeaklyTypedInput: true,
		TagName:          "consul",
	})
	if err != nil {
		return res, err
	}

	if err = decoder.Decode(raw); err != nil {
		return res, err
	}

	return res, nil
}

func newBackOff() backoff.BackOff {
	result := backoff.NewExponentialBackOff()
	result.InitialInterval = 1 * time.Second
	result.MaxInterval = 10 * time.Second
	result.MaxElapsedTime = 0
	return result
}
