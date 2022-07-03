package general

// Config - config for all master controllers.
type Config struct {
	Monitor struct {
		// timeoutConsulLeaderCheck - таймаут чека кто лидер через consul kv, (попытка забрать лидерство, если нет лидера)
		TimeoutConsulLeaderCheck string `yaml:"timeoutConsulLeaderCheck"`
	} `yaml:"monitor"`
}
