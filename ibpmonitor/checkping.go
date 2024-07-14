package ibpmonitor

import (
	"encoding/json"
	"fmt"
	"time"

	"ibp-geodns/config"

	"github.com/go-ping/ping"
)

type PingResult struct {
	CheckName  string
	ServerName string
	Success    bool
	Latency    time.Duration
	Error      string
}

func PingCheck(member Member, options config.CheckConfig, resultsCollectorChannel chan string) {
	checkName := "ping"

	pinger, err := ping.NewPinger(member.IPv4Address)
	if err != nil {
		err := fmt.Sprintf("Unable to launch ping: %v\n", err)
		resultsCollectorChannel <- fmt.Sprintf(`{"CheckName":"%s","ServerName":"%s","Success":false,"Error":"%s"}`, checkName, member.MemberName, err)
		return
	}

	pinger.Count = 3
	pinger.Timeout = time.Second * 2
	pinger.SetPrivileged(true)

	err = pinger.Run()
	if err != nil {
		err := fmt.Sprintf("Ping failed to run: %v\n", err)
		resultsCollectorChannel <- fmt.Sprintf(`{"CheckName":"%s","ServerName":"%s","Success":false,"Error":"%s"}`, checkName, member.MemberName, err)
		return
	}

	stats := pinger.Statistics()

	result := PingResult{
		CheckName:  checkName,
		ServerName: member.MemberName,
		Success:    true,
		Latency:    stats.AvgRtt,
	}

	resultJSON, _ := json.Marshal(result)
	resultsCollectorChannel <- string(resultJSON)
}

func init() {
	RegisterCheck("ping", PingCheck)
	RegisterResultType("ping", PingResult{})
}
