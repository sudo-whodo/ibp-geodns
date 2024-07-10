package ibpmonitor

import (
	"time"
)

func NewRpcHealth(members []Member, options Options) *RpcHealth {
	switch {
	case options.CheckInterval < 30:
		options.CheckInterval = 30 * time.Second // Default check interval
	case options.Timeout < 5:
		options.Timeout = 5 * time.Second // Default timeout
	}

	resultsChannel := make(chan string, 2)

	return &RpcHealth{
		Members:                 members,
		HealthStatus:            make(map[string]bool),
		StopChannel:             make(chan struct{}),
		options:                 options,
		ResultsChannel:          resultsChannel,
		NodeResults:             make(map[string]*NodeResults),
		ResultsCollectorChannel: make(chan string, len(members)*len(checks)+2),
	}
}

func (r *RpcHealth) LaunchChecks() {
	timer := time.NewTicker(r.options.CheckInterval)

	for {
		select {
		case <-timer.C:
			r.performChecks()
		case <-r.StopChannel:
			timer.Stop()
			return
		}
	}
}

func (r *RpcHealth) Start() chan string {
	go r.LaunchChecks()
	go r.MonitorResults()

	return r.ResultsChannel
}

func (r *RpcHealth) Stop() {
	close(r.StopChannel)
}
