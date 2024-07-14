package ibpmonitor

import (
	"ibp-geodns/config"
	"sync"
	"time"
)

func NewIbpMonitor(members []Member, config *config.Config) *IbpMonitor {
	resultsChannel := make(chan string, 2)

	return &IbpMonitor{
		Members:                 members,
		HealthStatus:            make(map[string]bool),
		StopChannel:             make(chan struct{}),
		Config:                  config,
		ResultsChannel:          resultsChannel,
		NodeResults:             make(map[string]*NodeResults),
		ResultsCollectorChannel: make(chan string, len(members)*len(config.Checks)+2),
	}
}

func (r *IbpMonitor) LaunchChecks() {
	var wg sync.WaitGroup

	for checkName, checkConfig := range r.Config.Checks {
		if checkConfig.Enabled == 1 {
			wg.Add(1)
			go func(checkName string, checkConfig config.CheckConfig) {
				defer wg.Done()

				r.performCheck(checkName)

				ticker := time.NewTicker(time.Duration(checkConfig.CheckInterval) * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						r.performCheck(checkName)
					case <-r.StopChannel:
						return
					}
				}
			}(checkName, checkConfig)
		}
	}

	wg.Wait()
}

func (r *IbpMonitor) Start() chan string {
	go r.LaunchChecks()
	go r.MonitorResults()

	return r.ResultsChannel
}

func (r *IbpMonitor) Stop() {
	close(r.StopChannel)
}
