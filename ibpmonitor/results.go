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
	interval := 1 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Println("Starting to monitor results")
	for {
		select {
		case result := <-r.ResultsCollectorChannel:
			// log.Printf("Received JSON: %s", result)
			go r.processResult(result)
		case <-ticker.C:
			go func() {
				jsonResults, err := r.sendBatchedResults()
				if err == nil {
					if jsonResults != "" {
						// Non-blocking send using select with default
						select {
						case r.ResultsChannel <- jsonResults:
							// Successfully sent
						default:
							// Handle the case where ResultsChannel is full
							log.Println("ResultsChannel is full. Dropping batched results.")
						}
					}
				} else {
					log.Printf("Error sending batched results: %v", err)
				}
			}()
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

	nodeResults, exists := r.NodeResults[memberName]
	if !exists {
		nodeResults = &NodeResults{Checks: make(map[string]interface{})}
		r.mu.Lock()
		r.NodeResults[memberName] = nodeResults
		r.mu.Unlock()
	}

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
	// Step 1: Lock and copy NodeResults
	r.mu.Lock()
	copiedNodeResults := make(map[string]*NodeResults, len(r.NodeResults))
	for memberName, nodeResult := range r.NodeResults {
		// Copy Checks
		copiedChecks := make(map[string]interface{}, len(nodeResult.Checks))
		for checkName, checkData := range nodeResult.Checks {
			copiedChecks[checkName] = checkData
		}

		// Copy EndpointChecks
		copiedEndpointChecks := make(map[string]map[string]interface{}, len(nodeResult.EndpointChecks))
		for endpointURL, checks := range nodeResult.EndpointChecks {
			copiedChecksMap := make(map[string]interface{}, len(checks))
			for checkName, checkData := range checks {
				copiedChecksMap[checkName] = checkData
			}
			copiedEndpointChecks[endpointURL] = copiedChecksMap
		}

		// Assign copied NodeResults
		copiedNodeResults[memberName] = &NodeResults{
			Checks:         copiedChecks,
			EndpointChecks: copiedEndpointChecks,
		}
	}
	r.mu.Unlock() // Unlock early to allow other operations

	// Step 3: Initialize batched results
	siteResults := config.SiteResults{
		ResultType: "site",
		Members:    make(map[string]map[string]config.SiteCheckResult),
	}

	endpointResults := config.EndpointResults{
		ResultType: "endpoint",
		Endpoint:   make(map[string]map[string]map[string]config.EndpointCheckResult),
	}

	// Step 4: Process copiedNodeResults
	for memberName, nodeResults := range copiedNodeResults {
		// Process site checks
		for checkName, resultData := range nodeResults.Checks {
			// Initialize member map if not present
			if siteResults.Members[memberName] == nil {
				siteResults.Members[memberName] = make(map[string]config.SiteCheckResult)
			}

			// Type assertion for resultData
			resultMap, ok := resultData.(map[string]interface{})
			if !ok {
				continue // Skip if type assertion fails
			}

			// Safely extract fields with type checks
			success, ok := resultMap["success"].(bool)
			if !ok {
				success = false // Default value if type assertion fails
			}
			checkError, ok := resultMap["error"].(string)
			if !ok {
				checkError = "" // Default value if type assertion fails
			}
			checkData, ok := resultMap["data"].(map[string]interface{})
			if !ok {
				checkData = nil // Default value if type assertion fails
			}

			// Assign to siteResults
			siteResults.Members[memberName][checkName] = config.SiteCheckResult{
				CheckName:  checkName,
				Success:    success,
				CheckError: checkError,
				CheckData:  checkData,
			}
		}

		// Process endpoint checks
		for endpointURL, checks := range nodeResults.EndpointChecks {
			// Initialize maps if not present
			if endpointResults.Endpoint[endpointURL] == nil {
				endpointResults.Endpoint[endpointURL] = make(map[string]map[string]config.EndpointCheckResult)
			}
			if endpointResults.Endpoint[endpointURL][memberName] == nil {
				endpointResults.Endpoint[endpointURL][memberName] = make(map[string]config.EndpointCheckResult)
			}

			for checkName, resultData := range checks {
				// Type assertion for resultData
				resultMap, ok := resultData.(map[string]interface{})
				if !ok {
					continue // Skip if type assertion fails
				}

				// Safely extract fields with type checks
				success, ok := resultMap["success"].(bool)
				if !ok {
					success = false // Default value if type assertion fails
				}
				checkError, ok := resultMap["error"].(string)
				if !ok {
					checkError = "" // Default value if type assertion fails
				}
				checkData, ok := resultMap["data"].(map[string]interface{})
				if !ok {
					checkData = nil // Default value if type assertion fails
				}

				// Assign to endpointResults
				endpointResults.Endpoint[endpointURL][memberName][checkName] = config.EndpointCheckResult{
					CheckName:  checkName,
					Success:    success,
					CheckError: checkError,
					CheckData:  checkData,
				}
			}
		}
	}

	// Step 5: Marshal the batched results to JSON
	siteResultsJSON, err := json.Marshal(siteResults)
	if err != nil {
		return "", fmt.Errorf("error marshaling site results: %v", err)
	}
	endpointResultsJSON, err := json.Marshal(endpointResults)
	if err != nil {
		return "", fmt.Errorf("error marshaling endpoint results: %v", err)
	}

	// Step 6: Concatenate the JSON results
	jsonResults := string(siteResultsJSON) + "\n" + string(endpointResultsJSON)

	// Step 7: Return the concatenated results
	return jsonResults, nil
}
