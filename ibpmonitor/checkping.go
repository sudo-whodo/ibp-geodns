package ibpmonitor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-ping/ping"
)

type PingResult struct {
	CheckName  string
	ServerName string
	Success    bool
	Latency    time.Duration
	Error      string
}

func PingCheck(server RpcServer, options Options, resultsCollectorChannel chan string) {
	checkName := "ping"
	done := make(chan PingResult, 2)
	timer := time.NewTimer(options.Timeout)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Sprintf("Ping check failed: %v", r)
				done <- PingResult{CheckName: checkName, ServerName: server.Name, Success: false, Error: err}
			}
			close(done)
		}()

		pinger, err := ping.NewPinger(server.Options.IpAddress)
		if err != nil {
			err := fmt.Sprintf("Unable to launch ping: %v\n", err)
			done <- PingResult{CheckName: checkName, ServerName: server.Name, Success: false, Error: err}
			return
		}

		pinger.Count = 3
		pinger.Timeout = time.Second * 2
		pinger.SetPrivileged(true)

		err = pinger.Run()
		if err != nil {
			err := fmt.Sprintf("Ping failed to run: %v\n", err)
			done <- PingResult{CheckName: checkName, ServerName: server.Name, Success: false, Latency: 0, Error: err}
			return
		}

		stats := pinger.Statistics()

		result := PingResult{
			CheckName:  checkName,
			ServerName: server.Name,
			Success:    true,
			Latency:    stats.AvgRtt,
		}

		done <- result
	}()

	select {
	case result := <-done:
		resultJSON, _ := json.Marshal(result)
		resultsCollectorChannel <- string(resultJSON)
	case <-timer.C:
		err := fmt.Sprintf("pingCheck for %s timed out", server.Name)
		resultJSON, _ := json.Marshal(PingResult{CheckName: checkName, ServerName: server.Name, Success: false, Latency: 0, Error: err})
		resultsCollectorChannel <- string(resultJSON)
	}
}

func init() {
	RegisterCheck("ping", PingCheck)
	RegisterResultType("ping", PingResult{})
}
