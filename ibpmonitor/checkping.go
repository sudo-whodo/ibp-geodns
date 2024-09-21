package ibpmonitor

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"ibp-geodns/config"

	"github.com/go-ping/ping"
)

type PingResult struct {
	CheckName  string   `json:"checkname"`
	ServerName string   `json:"servername"`
	ResultType string   `json:"resulttype"`
	Success    bool     `json:"success"`
	Error      string   `json:"error"`
	Data       PingData `json:"data"`
}

type PingData struct {
	Latency    int64   `json:"latency"`
	PacketLoss float64 `json:"packetloss"`
}

func PingCheck(member Member, options config.CheckConfig, resultsCollectorChannel chan string) {
	checkName := "ping"

	pingCount := getIntOption(options.ExtraOptions, "PingCount", 30)
	pingInterval := getIntOption(options.ExtraOptions, "PingInterval", 100)
	pingTimeout := getIntOption(options.ExtraOptions, "PingTimeout", 10000)
	pingTTL := getIntOption(options.ExtraOptions, "PingTTL", 255)
	pingSize := getIntOption(options.ExtraOptions, "PingSize", 32)
	maxPacketLoss := getIntOption(options.ExtraOptions, "MaxPacketLoss", 5)
	maxLatency := getIntOption(options.ExtraOptions, "MaxLatency", 800)

	pinger, err := ping.NewPinger(member.IPv4Address)
	if err != nil {
		err := fmt.Sprintf("Unable to launch ping: %v\n", err)
		result := PingResult{
			CheckName:  checkName,
			ServerName: member.MemberName,
			ResultType: "site",
			Success:    false,
			Error:      err,
		}

		finalResultJSON, _ := json.Marshal(result)
		// log.Printf("Generated JSON (error): %s", finalResultJSON)
		resultsCollectorChannel <- string(finalResultJSON)
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
			ResultType: "site",
			Success:    false,
			Error:      err,
		}

		finalResultJSON, _ := json.Marshal(result)
		// log.Printf("Generated JSON (error): %s", finalResultJSON)
		resultsCollectorChannel <- string(finalResultJSON)
		return
	}

	stats := pinger.Statistics()

	success := stats.PacketLoss <= float64(maxPacketLoss) && stats.AvgRtt.Milliseconds() <= int64(maxLatency) && stats.AvgRtt != 0

	if !success {
		log.Printf("Member: %s failed ping check - Packet Loss: %v (Max: %v) Latency: %d (Max: %d)", member.MemberName, stats.PacketLoss, float64(maxPacketLoss), stats.AvgRtt.Milliseconds(), int64(maxLatency))
	}

	result := PingResult{
		CheckName:  checkName,
		ServerName: member.MemberName,
		ResultType: "site",
		Success:    success,
		Data: PingData{
			Latency:    stats.AvgRtt.Milliseconds(),
			PacketLoss: stats.PacketLoss,
		},
	}

	finalResultJSON, _ := json.Marshal(result)
	// log.Printf("Generated JSON: %s", finalResultJSON)
	resultsCollectorChannel <- string(finalResultJSON)
}

func init() {
	RegisterCheck("ping", PingCheck)
	RegisterResultType("ping", PingResult{})
}
