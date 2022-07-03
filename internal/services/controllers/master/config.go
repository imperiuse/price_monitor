package master

// Config - config for all master controllers.
type Config struct {
	Scanner struct {
		// TimeoutOneTaskProcess - timeout for one task process
		TimeoutOneTaskProcess string `yaml:"timeoutOneTaskProcess"`

		// IntervalPeriodicScan - scan interval
		IntervalPeriodicScan string `yaml:"intervalPeriodicScan"`

		// CntScanWorkers - cnt of workers
		CntWorkers int `yaml:"cntWorkers"`
	} `yaml:"scanner"`
}
