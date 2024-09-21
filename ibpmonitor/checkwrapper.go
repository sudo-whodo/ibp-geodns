package ibpmonitor

import (
	"encoding/json"
	"fmt"
	"ibp-geodns/config"
	"time"
)

type EndpointCheck func(member Member, options config.CheckConfig, resultsCollectorChannel chan string, endpointURL string)

// It issues results to the resultsCollectorChannel based on whether the check is a site or endpoint check.
func CheckWrapper(checkName string, checkFunc Check, member Member, options config.CheckConfig, resultsCollectorChannel chan string) {
	done := make(chan struct{})
	timer := time.NewTimer(time.Duration(options.Timeout) * time.Second)

	isEndpointCheck := options.CheckType == "endpoint"

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("%s check failed for member %s: %v", checkName, member.MemberName, r)
				if isEndpointCheck {
					// For endpoint checks, issue a result for every endpoint
					endpoints := collectEndpoints(member)
					for _, endpointURL := range endpoints {
						sendResult(checkName, member.MemberName, endpointURL, "endpoint", false, errMsg, resultsCollectorChannel)
					}
				} else {
					// For site checks, issue a single result
					sendResult(checkName, member.MemberName, "", "site", false, errMsg, resultsCollectorChannel)
				}
			}
			close(done)
		}()

		// Call the actual check function
		checkFunc(member, options, resultsCollectorChannel)
	}()

	select {
	case <-done:
		// Check completed
	case <-timer.C:
		errMsg := fmt.Sprintf("%s check for member %s timed out", checkName, member.MemberName)
		if isEndpointCheck {
			// For endpoint checks, issue a result for every endpoint
			endpoints := collectEndpoints(member)
			for _, endpointURL := range endpoints {
				sendResult(checkName, member.MemberName, endpointURL, "endpoint", false, errMsg, resultsCollectorChannel)
			}
		} else {
			// For site checks, issue a single result
			sendResult(checkName, member.MemberName, "", "site", false, errMsg, resultsCollectorChannel)
		}
	}
}

// sendResult constructs and sends the result to the resultsCollectorChannel.
func sendResult(checkName, memberName, endpointURL, resultType string, success bool, errMsg string, resultsCollectorChannel chan string) {
	result := map[string]interface{}{
		"checkname":  checkName,
		"membername": memberName,
		"resulttype": resultType,
		"success":    success,
		"error":      errMsg,
		"data":       map[string]interface{}{},
	}
	if resultType == "endpoint" {
		result["endpointurl"] = endpointURL
	}
	resultJSON, _ := json.Marshal(result)
	resultsCollectorChannel <- string(resultJSON)
}

// collectEndpoints collects all unique endpoints for a member.
func collectEndpoints(member Member) []string {
	uniqueEndpoints := make(map[string]bool)
	for _, service := range member.Services {
		for _, endpoint := range service.Endpoints {
			uniqueEndpoints[endpoint] = true
		}
	}
	endpoints := make([]string, 0, len(uniqueEndpoints))
	for endpoint := range uniqueEndpoints {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}
