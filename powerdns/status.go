package powerdns

import (
	"encoding/json"
	"fmt"
	"ibp-geodns/config"
	"ibp-geodns/matrixbot"
	"log"
	"strings"
	"sync"
)

var previousStatus = make(map[string]map[string]map[string]bool)
var mu sync.RWMutex

func updateMemberStatus() {
	for result := range resultsChannel {
		// log.Printf("Received result: %s", result) // Log the received result

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

	for memberName, checks := range status.Members {
		for checkName, result := range checks {
			if previousStatus["site"] == nil {
				previousStatus["site"] = make(map[string]map[string]bool)
			}
			if previousStatus["site"][memberName] == nil {
				previousStatus["site"][memberName] = make(map[string]bool)
			}
			if prevSuccess, exists := previousStatus["site"][memberName][checkName]; !exists || prevSuccess != result.Success {
				bot, err := matrixbot.NewMatrixBot(configData.Matrix.HomeServerURL, configData.Matrix.Username, configData.Matrix.Password, configData.Matrix.RoomID)
				if err != nil {
					log.Printf("Error initializing Matrix bot: %v", err)
				} else {
					message := fmt.Sprintf("<b>Site Status Change</b><br><i><b>Server:</b> %s</i><br><i><b>Member:</b> %s</i><br><i><b>Check %s:</b> %v -> %v</i><BR><b>Result Data:</b> %v", configData.ServerName, memberName, checkName, prevSuccess, result.Success, result.CheckData)
					go bot.SendMessage(message)
				}

				log.Printf("Site Status Change: Server %s member %s - check %s: %v -> %v - Result Data: %v", configData.ServerName, memberName, checkName, prevSuccess, result.Success, result.CheckData)

				previousStatus["site"][memberName][checkName] = result.Success
			}

			for i, config := range powerDNSConfigs {
				if member, exists := config.Members[memberName]; exists {
					if member.Results == nil {
						member.Results = make(map[string]Result)
					}
					member.Results[checkName] = Result{Success: result.Success}
					powerDNSConfigs[i].Members[memberName] = member
				}
			}
		}
	}
}

func updateEndpointStatus(status config.EndpointResults) {
	mu.Lock()
	defer mu.Unlock()

	for endpointURL, members := range status.Endpoint {
		for memberName, checks := range members {
			for checkName, result := range checks {
				if previousStatus[endpointURL] == nil {
					previousStatus[endpointURL] = make(map[string]map[string]bool)
				}
				if previousStatus[endpointURL][memberName] == nil {
					previousStatus[endpointURL][memberName] = make(map[string]bool)
				}
				if prevSuccess, exists := previousStatus[endpointURL][memberName][checkName]; !exists || prevSuccess != result.Success {
					bot, err := matrixbot.NewMatrixBot(configData.Matrix.HomeServerURL, configData.Matrix.Username, configData.Matrix.Password, configData.Matrix.RoomID)
					if err != nil {
						log.Printf("Error initializing Matrix bot: %v", err)
					} else {
						message := fmt.Sprintf("<b>EndPoint Status Change</b><br><i><b>Server:</b> %s</i><br><i><b>Member:</b> %s <b>Domain:</b> %s</i><br><i><b>Check %s:</b> %v -> %v</i><BR><b>Result Data:</b> %v", configData.ServerName, memberName, endpointURL, checkName, prevSuccess, result.Success, result.CheckData)
						go bot.SendMessage(message)
					}

					log.Printf("EndPoint Status Change: Server %s - member %s on %s - Check %s: %v -> %v - Result Data: %v", configData.ServerName, memberName, endpointURL, checkName, prevSuccess, result.Success, result.CheckData)

					previousStatus[endpointURL][memberName][checkName] = result.Success
				}

				for i, config := range powerDNSConfigs {
					if config.Domain == endpointURL {
						if member, exists := config.Members[memberName]; exists {
							if member.Results == nil {
								member.Results = make(map[string]Result)
							}
							member.Results[checkName] = Result{Success: result.Success}
							powerDNSConfigs[i].Members[memberName] = member
						}
					}
				}
			}
		}
	}
}
