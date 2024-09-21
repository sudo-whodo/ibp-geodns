package ibpmonitor

import (
	"encoding/json"
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
	interval := 10 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Println("Starting to monitor results")
	for {
		select {
		case result := <-r.ResultsCollectorChannel:
			// log.Printf("Received JSON: %s", result)
			r.processResult(result)
		case <-ticker.C:
			jsonResults, err := r.sendBatchedResults()
			if err == nil {
				if jsonResults != "" {
					// Send the batched results through the ResultsChannel
					r.ResultsChannel <- jsonResults
				}
			} else {
				log.Printf("Error sending batched results: %v", err)
			}
		}
	}
}

func (r *IbpMonitor) processResult(result string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		log.Printf("Error unmarshaling result: %v", err)
		return
	}

	resultType, ok := data["resulttype"].(string)
	if !ok {
		//log.Printf("Error: resulttype not found or not a string in result: %v", data)
		return
	}

	switch resultType {
	case "site":
		r.processSiteResult(data)
	case "endpoint":
		r.processEndpointResult(data)
	default:
		//log.Printf("Unknown resulttype '%s' in result: %v", resultType, data)
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
	// Extract memberName from the result data
	memberName, ok := data["membername"].(string)
	if !ok {
		log.Printf("Error: membername not found or not a string in result: %v", data)
		return
	}

	// Extract checkName from the result data
	checkName, ok := data["checkname"].(string)
	if !ok {
		log.Printf("Error: checkname not found or not a string in result: %v", data)
		return
	}

	// Extract endpointURL from the result data
	endpointURL, ok := data["endpointurl"].(string)
	if !ok {
		log.Printf("Error: endpointurl not found or not a string in result: %v", data)
		return
	}

	// Lock the IbpMonitor's NodeResults map for thread-safe access
	r.mu.Lock()
	nodeResults, exists := r.NodeResults[memberName]
	if !exists {
		// Initialize NodeResults for this member if it doesn't exist
		nodeResults = &NodeResults{
			Checks:         make(map[string]interface{}),
			EndpointChecks: make(map[string]map[string]interface{}),
		}
		r.NodeResults[memberName] = nodeResults
	}
	r.mu.Unlock()

	// Lock the NodeResults for this member
	nodeResults.mu.Lock()
	defer nodeResults.mu.Unlock()

	// Ensure the EndpointChecks map is initialized
	if nodeResults.EndpointChecks == nil {
		nodeResults.EndpointChecks = make(map[string]map[string]interface{})
	}

	// Ensure the map for this endpointURL exists
	if nodeResults.EndpointChecks[endpointURL] == nil {
		nodeResults.EndpointChecks[endpointURL] = make(map[string]interface{})
	}

	// Store the check result
	nodeResults.EndpointChecks[endpointURL][checkName] = data

	// Mark the check as completed in nodeResults.Checks
	nodeResults.Checks[checkName] = true

	// Log the updated NodeResults for debugging
	//log.Printf("Updated NodeResults for member '%s': %+v", memberName, nodeResults)
}

func (r *IbpMonitor) sendBatchedResults() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize batched results
	siteResults := config.SiteResults{
		ResultType: "site",
		Members:    make(map[string]map[string]config.SiteCheckResult),
	}

	endpointResults := config.EndpointResults{
		ResultType: "endpoint",
		Endpoint:   make(map[string]map[string]map[string]config.EndpointCheckResult),
	}

	for memberName, nodeResults := range r.NodeResults {
		// Process site checks
		for checkName, resultData := range nodeResults.Checks {
			// Assuming siteResults.Members[memberName] is initialized
			if siteResults.Members[memberName] == nil {
				siteResults.Members[memberName] = make(map[string]config.SiteCheckResult)
			}
			// Cast resultData to the appropriate type if needed
			resultMap, ok := resultData.(map[string]interface{})
			if !ok {
				continue
			}
			siteResults.Members[memberName][checkName] = config.SiteCheckResult{
				CheckName:  checkName,
				Success:    resultMap["success"].(bool),
				CheckError: resultMap["error"].(string),
				CheckData:  resultMap["data"].(map[string]interface{}),
			}
		}

		// Process endpoint checks
		for endpointURL, checks := range nodeResults.EndpointChecks {
			if endpointResults.Endpoint[endpointURL] == nil {
				endpointResults.Endpoint[endpointURL] = make(map[string]map[string]config.EndpointCheckResult)
			}
			if endpointResults.Endpoint[endpointURL][memberName] == nil {
				endpointResults.Endpoint[endpointURL][memberName] = make(map[string]config.EndpointCheckResult)
			}
			for checkName, resultData := range checks {
				resultMap, ok := resultData.(map[string]interface{})
				if !ok {
					continue
				}
				endpointResults.Endpoint[endpointURL][memberName][checkName] = config.EndpointCheckResult{
					CheckName:  checkName,
					Success:    resultMap["success"].(bool),
					CheckError: resultMap["error"].(string),
					CheckData:  resultMap["data"].(map[string]interface{}),
				}
			}
		}
	}

	// Marshal the batched results
	siteResultsJSON, err := json.Marshal(siteResults)
	if err != nil {
		return "", err
	}
	endpointResultsJSON, err := json.Marshal(endpointResults)
	if err != nil {
		return "", err
	}

	// Concatenate the JSON results
	jsonResults := string(siteResultsJSON) + "\n" + string(endpointResultsJSON)

	// Log the batched results before sending
	//log.Printf("Sending batched site results: %s", string(siteResultsJSON))
	//log.Printf("Sending batched endpoint results: %s", string(endpointResultsJSON))

	// Clear NodeResults after sending
	r.NodeResults = make(map[string]*NodeResults)

	// Return the concatenated results
	return jsonResults, nil
}
