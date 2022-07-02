package storage

type (
	// Config - config of storage.
	Config struct {
		ConsulLeaderKeyPrefix    string `yaml:"consul_leader_key_prefix"`
		ConsulSlaveListKeyPrefix string `yaml:"consul_slave_list_key_prefix"`
		Host                     string // todo in real world array of hosts or slaves or other // can use Consul
		Username                 string
		Password                 string
		Database                 string
		Port                     int
		Options                  Options
	}

	// Options - options config.
	Options map[string]any

	// Storage - interface desc DB Storage
	Storage interface {
		Connect()
		Close()
	}
)
