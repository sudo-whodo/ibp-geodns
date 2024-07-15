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
	CheckType  string
	Success    bool
	Latency    time.Duration
	PacketLoss float64
	Error      string
}

func getIntOption(extraOptions map[string]interface{}, key string, defaultValue int) int {
	if value, ok := extraOptions[key].(float64); ok {
		return int(value)
	}
	return defaultValue
}

func PingCheck(member Member, options config.CheckConfig, resultsCollectorChannel chan string) {
	checkName := "ping"
	checkType := "site"

	pingCount := getIntOption(options.ExtraOptions, "PingCount", 10)
	pingInterval := getIntOption(options.ExtraOptions, "PingInterval", 100)
	pingTimeout := getIntOption(options.ExtraOptions, "PingTimeout", 2000)
	pingTTL := getIntOption(options.ExtraOptions, "PingTTL", 64)
	pingSize := getIntOption(options.ExtraOptions, "PingSize", 24)

	pinger, err := ping.NewPinger(member.IPv4Address)
	if err != nil {
		err := fmt.Sprintf("Unable to launch ping: %v\n", err)
		result := PingResult{
			CheckName:  checkName,
			ServerName: member.MemberName,
			CheckType:  checkType,
			Success:    false,
			Latency:    0,
			Error:      err,
		}

		// log.Printf("Ping Result: %+v", result)

		resultJSON, _ := json.Marshal(result)
		resultsCollectorChannel <- string(resultJSON)
		return
	}

	pinger.Count = pingCount
	pinger.Timeout = time.Duration(pingTimeout * int(time.Millisecond))
	pinger.TTL = pingTTL
	pinger.Interval = time.Duration(pingInterval * int(time.Millisecond))
	pinger.Size = pingSize
	pinger.SetPrivileged(true)

	err = pinger.Run()
	if err != nil {
		err := fmt.Sprintf("Ping failed to run: %v\n", err)
		result := PingResult{
			CheckName:  checkName,
			ServerName: member.MemberName,
			CheckType:  checkType,
			Success:    false,
			Latency:    0,
			Error:      err,
		}

		// log.Printf("Ping Result: %+v", result)

		resultJSON, _ := json.Marshal(result)
		resultsCollectorChannel <- string(resultJSON)
		return
	}

	stats := pinger.Statistics()

	// log.Printf("Ping Statistics: %+v", stats)

	success := stats.PacketsRecv == stats.PacketsSent

	result := PingResult{
		CheckName:  checkName,
		ServerName: member.MemberName,
		CheckType:  checkType,
		Success:    success,
		Latency:    stats.AvgRtt,
		PacketLoss: stats.PacketLoss,
	}

	// log.Printf("Ping Result: %+v", result)

	resultJSON, _ := json.Marshal(result)
	resultsCollectorChannel <- string(resultJSON)
}

func init() {
	RegisterCheck("ping", PingCheck)
	RegisterResultType("ping", PingResult{})
}
