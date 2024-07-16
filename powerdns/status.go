package powerdns

import (
	"encoding/json"
	"ibp-geodns/config"
	"log"
	"strings"
	"sync"
)

var previousStatus = make(map[string]bool)
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
			if prevSuccess, exists := previousStatus[memberName]; !exists || prevSuccess != result.Success {
				log.Printf("Site status change for member %s, check %s: %v -> %v - Result Data: %v", memberName, checkName, prevSuccess, result.Success, result.CheckData)
				previousStatus[memberName] = result.Success
			}

			for i, config := range powerDNSConfigs {
				if member, exists := config.Members[memberName]; exists {
					member.Online = result.Success
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
				if prevSuccess, exists := previousStatus[memberName]; !exists || prevSuccess != result.Success {
					log.Printf("Endpoint status change for endpoint %s, member %s, check %s: %v -> %v - Result Data: %v", endpointURL, memberName, checkName, prevSuccess, result.Success, result.CheckData)
					previousStatus[memberName] = result.Success
				}

				for i, config := range powerDNSConfigs {
					if member, exists := config.Members[memberName]; exists {
						member.Online = result.Success
						powerDNSConfigs[i].Members[memberName] = member
					}
				}
			}
		}
	}
}
