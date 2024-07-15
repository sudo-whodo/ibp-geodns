package ibpmonitor

import (
	"encoding/json"
	"fmt"
	"ibp-geodns/config"
	"log"
	"sync"
	"time"
)

var (
	resultTypes      = make(map[string]interface{})
	resultTypesMutex sync.Mutex
)

func RegisterResultType(name string, resultType interface{}) {
	resultTypesMutex.Lock()
	defer resultTypesMutex.Unlock()
	resultTypes[name] = resultType
}

func GetResultType(name string) (interface{}, bool) {
	resultTypesMutex.Lock()
	defer resultTypesMutex.Unlock()
	resultType, exists := resultTypes[name]
	return resultType, exists
}

func (r *IbpMonitor) MonitorResults() {
	interval := 100 * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	adjustTicker := func(newInterval time.Duration) {
		interval = newInterval
		ticker.Stop()
		ticker = time.NewTicker(interval)
	}

	log.Println("Starting to monitor results")
	for {
		select {
		case result := <-r.ResultsCollectorChannel:
			// log.Printf("Received JSON: %s", result)
			r.processResult(result)
		case <-ticker.C:
			if r.allChecksComplete() {
				jsonResults, err := r.sendBatchedResults()
				if err == nil {
					if jsonResults != "" {
						//log.Printf("Sending batched results: %s", jsonResults) // Log the batched results being sent
						r.ResultsChannel <- jsonResults
					}
					if interval == 100*time.Millisecond {
						adjustTicker(3 * time.Second)
					}
				} else {
					log.Printf("Error sending batched results: %v", err)
				}
			}
		}
	}
}

func (r *IbpMonitor) processResult(resultJSON string) {
	var tempResult map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &tempResult); err != nil {
		log.Printf("Error unmarshalling result: %v", err)
		return
	}

	resultType, ok := tempResult["resulttype"].(string)
	if !ok {
		log.Printf("Error: resulttype not found or not a string in result: %v", tempResult)
		return
	}

	switch resultType {
	case "site":
		r.processSiteResult(tempResult)
	case "endpoint":
		r.processEndpointResult(tempResult)
	default:
		log.Printf("Unknown result type: %s", resultType)
	}
}

func (r *IbpMonitor) processSiteResult(data map[string]interface{}) {
	memberName, ok := data["servername"].(string)
	if !ok {
		log.Printf("Error: ServerName not found or not a string in result: %v", data)
		return
	}

	checkName, ok := data["checkname"].(string)
	if !ok {
		log.Printf("Error: CheckName not found or not a string in result: %v", data)
		return
	}

	r.mu.Lock()
	nodeResults, exists := r.NodeResults[memberName]
	if !exists {
		nodeResults = &NodeResults{Checks: make(map[string]interface{})}
		r.NodeResults[memberName] = nodeResults
	}
	r.mu.Unlock()

	nodeResults.mu.Lock()
	nodeResults.Checks[checkName] = data
	nodeResults.mu.Unlock()
}

func (r *IbpMonitor) processEndpointResult(data map[string]interface{}) {
	memberName, ok := data["servername"].(string)
	if !ok {
		log.Printf("Error: ServerName not found or not a string in result: %v", data)
		return
	}

	checkName, ok := data["checkname"].(string)
	if !ok {
		log.Printf("Error: CheckName not found or not a string in result: %v", data)
		return
	}

	r.mu.Lock()
	nodeResults, exists := r.NodeResults[memberName]
	if !exists {
		nodeResults = &NodeResults{Checks: make(map[string]interface{})}
		r.NodeResults[memberName] = nodeResults
	}
	r.mu.Unlock()

	nodeResults.mu.Lock()
	nodeResults.Checks[checkName] = data
	nodeResults.mu.Unlock()
}

func (r *IbpMonitor) sendBatchedResults() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	siteResults := config.SiteResults{
		ResultType: "site",
		Members:    make(map[string]map[string]config.SiteCheckResult),
	}

	endpointResults := config.EndpointResults{
		ResultType: "endpoint",
		Endpoint:   make(map[string]map[string]map[string]config.EndpointCheckResult),
	}

	for memberName, resultsStruct := range r.NodeResults {
		for checkName, checkResult := range resultsStruct.Checks {
			checkResultMap, ok := checkResult.(map[string]interface{})
			if !ok {
				log.Printf("Error converting checkResult to map for member %s, check %s", memberName, checkName)
				continue
			}

			switch checkResultMap["resulttype"] {
			case "site":
				if siteResults.Members[memberName] == nil {
					siteResults.Members[memberName] = make(map[string]config.SiteCheckResult)
				}
				siteResults.Members[memberName][checkName] = config.SiteCheckResult{
					CheckName:  checkName,
					Success:    checkResultMap["success"].(bool),
					CheckError: checkResultMap["error"].(string),
					CheckData:  checkResultMap["data"].(map[string]interface{}),
				}
			case "endpoint":
				endpointURL := checkResultMap["servername"].(string) // Adjust this to your actual endpoint identification
				if endpointResults.Endpoint[endpointURL] == nil {
					endpointResults.Endpoint[endpointURL] = make(map[string]map[string]config.EndpointCheckResult)
				}
				if endpointResults.Endpoint[endpointURL][memberName] == nil {
					endpointResults.Endpoint[endpointURL][memberName] = make(map[string]config.EndpointCheckResult)
				}
				endpointResults.Endpoint[endpointURL][memberName][checkName] = config.EndpointCheckResult{
					CheckName:  checkName,
					Success:    checkResultMap["success"].(bool),
					CheckError: checkResultMap["error"].(string),
					CheckData:  checkResultMap["data"].(map[string]interface{}),
				}
			}
		}
	}

	siteResultsJSON, err := json.Marshal(siteResults)
	if err != nil {
		log.Printf("Error marshalling site results: %v", err)
		return "", err
	}

	endpointResultsJSON, err := json.Marshal(endpointResults)
	if err != nil {
		log.Printf("Error marshalling endpoint results: %v", err)
		return "", err
	}

	batchedResults := fmt.Sprintf("%s\n%s", string(siteResultsJSON), string(endpointResultsJSON))
	// log.Printf("Generated batched results: %s", batchedResults)
	return batchedResults, nil
}

func (r *IbpMonitor) allChecksComplete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	var checksToValidate []string
	for checkName, checkConfig := range r.Config.Checks {
		if checkConfig.Enabled == 1 {
			checksToValidate = append(checksToValidate, checkName)
		}
	}

	for _, nodeResults := range r.NodeResults {
		for _, checkName := range checksToValidate {
			if _, exists := nodeResults.Checks[checkName]; !exists {
				return false
			}
		}
	}

	return true
}
