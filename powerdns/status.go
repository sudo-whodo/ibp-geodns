package powerdns

import (
	"encoding/json"
	"fmt"
	"ibp-geodns/config"
	"ibp-geodns/matrixbot"
	"log"
	"strings"
	"sync"
	"time"
)

var previousStatus = make(map[string]map[string]map[string]bool)
var mu sync.RWMutex

func updateMemberStatus() {
	for result := range resultsChannel {
		// log.Printf("Received result: %s", result)

		results := strings.Split(result, "\n")
		for _, res := range results {
			if strings.TrimSpace(res) == "" {
				continue
			}

			var siteStatus config.SiteResults
			var endpointStatus config.EndpointResults

			if err := json.Unmarshal([]byte(res), &siteStatus); err == nil && siteStatus.ResultType == "site" {
				updateSiteStatus(siteStatus)
			} else if err := json.Unmarshal([]byte(res), &endpointStatus); err == nil && endpointStatus.ResultType == "endpoint" {
				updateEndpointStatus(endpointStatus)
			} else {
				log.Printf("Error parsing result: %v", err)
			}
		}
	}
}

func updateSiteStatus(status config.SiteResults) {
	mu.Lock()
	defer mu.Unlock()

	// log.Printf("Updating site status: %+v", status)

	for memberName, checks := range status.Members {
		for checkName, result := range checks {
			if previousStatus["site"] == nil {
				previousStatus["site"] = make(map[string]map[string]bool)
			}
			if previousStatus["site"][memberName] == nil {
				previousStatus["site"][memberName] = make(map[string]bool)
			}

			member, memberExists := getMember(memberName)
			if !memberExists {
				continue
			}

			if previousStatus["site"][memberName][checkName] != result.Success {
				// log.Printf("Status change detected for site member %s check %s: %v -> %v", memberName, checkName, previousStatus["site"][memberName][checkName], result.Success)

				if member.Results == nil {
					member.Results = make(map[string]Result)
				}

				if result.Success {
					if member.Results[checkName].OfflineTS.IsZero() {
						member.Results[checkName] = Result{Success: true}
						previousStatus["site"][memberName][checkName] = result.Success
					} else if time.Since(member.Results[checkName].OfflineTS).Seconds() <= float64(configData.MinimumOfflineTime) {
						continue
					}

					if !member.Results[checkName].OfflineTS.IsZero() && time.Since(member.Results[checkName].OfflineTS).Seconds() >= float64(configData.MinimumOfflineTime) {
						member.Results[checkName] = Result{Success: true}
						previousStatus["site"][memberName][checkName] = result.Success
						sendMatrixMessage(fmt.Sprintf("<b>Adding member</b> <i>%s</i> <b>to all rotations</b><br><i><b>Server:</b> %s</i><br><i><b>Check %s:</b> false -> true</i><BR><b>Result Data:</b> %v", memberName, configData.ServerName, checkName, result.CheckData))
						logStatusChange("Site Status Change", memberName, checkName, false, true, result.CheckData)
					}
				} else {
					member.Results[checkName] = Result{
						Success:   false,
						OfflineTS: time.Now(),
					}
					previousStatus["site"][memberName][checkName] = result.Success
					sendMatrixMessage(fmt.Sprintf("<b>Removing member</b> <i>%s</i> <b>from all rotations</b><br><i><b>Server:</b> %s</i><br><i><b>Check %s:</b> true -> false</i><BR><b>Result Data:</b> %v", memberName, configData.ServerName, checkName, result.CheckData))
					logStatusChange("Site Status Change", memberName, checkName, true, false, result.CheckData)
				}

				for i := range powerDNSConfigs {
					if _, memberExists := powerDNSConfigs[i].Members[memberName]; memberExists {
						powerDNSConfigs[i].Members[memberName] = member
					}
				}
			} else {
				if !result.Success {
					if member.Results == nil {
						member.Results = make(map[string]Result)
					}
					if member.Results[checkName].Success || time.Since(member.Results[checkName].OfflineTS).Seconds() > float64(configData.MinimumOfflineTime) {
						member.Results[checkName] = Result{
							Success:   false,
							OfflineTS: time.Now(),
						}
						for i := range powerDNSConfigs {
							if _, memberExists := powerDNSConfigs[i].Members[memberName]; memberExists {
								powerDNSConfigs[i].Members[memberName] = member
							}
						}
					}
				}
			}
		}
	}
}

func updateEndpointStatus(status config.EndpointResults) {
	mu.Lock()
	defer mu.Unlock()

	// log.Printf("Updating endpoint status: %+v", status)

	for endpointURL, members := range status.Endpoint {
		for memberName, checks := range members {
			for checkName, result := range checks {
				if previousStatus[endpointURL] == nil {
					previousStatus[endpointURL] = make(map[string]map[string]bool)
				}
				if previousStatus[endpointURL][memberName] == nil {
					previousStatus[endpointURL][memberName] = make(map[string]bool)
				}

				member, memberExists := getMember(memberName)
				if !memberExists {
					continue
				}

				if previousStatus[endpointURL][memberName][checkName] != result.Success {
					// log.Printf("Status change detected for endpoint member %s check %s: %v -> %v", memberName, checkName, previousStatus[endpointURL][memberName][checkName], result.Success)

					if member.Results == nil {
						member.Results = make(map[string]Result)
					}

					if result.Success {
						if member.Results[checkName].OfflineTS.IsZero() {
							member.Results[checkName] = Result{Success: true}
							previousStatus[endpointURL][memberName][checkName] = result.Success
						} else if time.Since(member.Results[checkName].OfflineTS).Seconds() <= float64(configData.MinimumOfflineTime) {
							continue
						}

						if !member.Results[checkName].OfflineTS.IsZero() && time.Since(member.Results[checkName].OfflineTS).Seconds() >= float64(configData.MinimumOfflineTime) {
							member.Results[checkName] = Result{Success: true}
							previousStatus[endpointURL][memberName][checkName] = result.Success
							sendMatrixMessage(fmt.Sprintf("<b>Adding member</b> <i>%s</i> <b>to endpoint</b> <i>%s</i><br><i><b>Server:</b> %s</i><br><i><b>Check %s:</b> false -> true</i><BR><b>Result Data:</b> %v", memberName, endpointURL, configData.ServerName, checkName, result.CheckData))
							logStatusChange("EndPoint Status Change", memberName, checkName, false, true, result.CheckData)
						}
					} else {
						member.Results[checkName] = Result{
							Success:   false,
							OfflineTS: time.Now(),
						}
						previousStatus[endpointURL][memberName][checkName] = result.Success
						sendMatrixMessage(fmt.Sprintf("<b>Removing member</b> <i>%s</i> <b>from endpoint</b> <i>%s</i><br><i><b>Server:</b> %s</i><br><i><b>Check %s:</b> true -> false</i><BR><b>Result Data:</b> %v", memberName, endpointURL, configData.ServerName, checkName, result.CheckData))
						logStatusChange("EndPoint Status Change", memberName, checkName, true, false, result.CheckData)
					}

					for j := range powerDNSConfigs {
						if powerDNSConfigs[j].Domain == endpointURL {
							if _, memberExists := powerDNSConfigs[j].Members[memberName]; memberExists {
								powerDNSConfigs[j].Members[memberName] = member
							}
						}
					}
				} else {
					if !result.Success {
						if member.Results == nil {
							member.Results = make(map[string]Result)
						}
						if member.Results[checkName].Success || time.Since(member.Results[checkName].OfflineTS).Seconds() > float64(configData.MinimumOfflineTime) {
							member.Results[checkName] = Result{
								Success:   false,
								OfflineTS: time.Now(),
							}
							for j := range powerDNSConfigs {
								if powerDNSConfigs[j].Domain == endpointURL {
									if _, memberExists := powerDNSConfigs[j].Members[memberName]; memberExists {
										powerDNSConfigs[j].Members[memberName] = member
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func getMember(memberName string) (Member, bool) {
	for _, config := range powerDNSConfigs {
		if member, exists := config.Members[memberName]; exists {
			if member.Results == nil {
				member.Results = make(map[string]Result)
			}
			return member, true
		}
	}
	return Member{}, false
}

func sendMatrixMessage(message string) {
	bot, err := matrixbot.NewMatrixBot(configData.Matrix.HomeServerURL, configData.Matrix.Username, configData.Matrix.Password, configData.Matrix.RoomID)
	if err != nil {
		log.Printf("Error initializing Matrix bot: %v", err)
	} else {
		go bot.SendMessage(message)
	}
}

func logStatusChange(changeType, memberName, checkName string, prevSuccess, newSuccess bool, resultData interface{}) {
	log.Printf("%s: Server %s - member %s - Check %s: %v -> %v - Result Data: %v", changeType, configData.ServerName, memberName, checkName, prevSuccess, newSuccess, resultData)
}

func formatPowerDNSConfigs() string {
	var sb strings.Builder
	for _, config := range powerDNSConfigs {
		sb.WriteString(fmt.Sprintf("Domain: %s\n", config.Domain))
		for memberName, member := range config.Members {
			sb.WriteString(fmt.Sprintf("  Member: %s\n", memberName))
			sb.WriteString(fmt.Sprintf("    IPv4: %s\n", member.IPv4))
			sb.WriteString(fmt.Sprintf("    IPv6: %s\n", member.IPv6))
			sb.WriteString(fmt.Sprintf("    Latitude: %f\n", member.Latitude))
			sb.WriteString(fmt.Sprintf("    Longitude: %f\n", member.Longitude))
			sb.WriteString(fmt.Sprintf("    Results:\n"))
			for checkName, result := range member.Results {
				sb.WriteString(fmt.Sprintf("      Check: %s\n", checkName))
				sb.WriteString(fmt.Sprintf("        Success: %v\n", result.Success))
				if !result.OfflineTS.IsZero() {
					sb.WriteString(fmt.Sprintf("        OfflineTS: %s\n", result.OfflineTS))
				}
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func sendPowerDNSConfigsToMatrix() {
	message := formatPowerDNSConfigs()
	sendMatrixMessage(message)
}

func startConfigReportTicker() {
	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		for {
			<-ticker.C
			sendPowerDNSConfigsToMatrix()
		}
	}()
}
