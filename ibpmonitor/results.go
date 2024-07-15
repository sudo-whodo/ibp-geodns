package ibpmonitor

import (
	"encoding/json"
	"fmt"
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
		// log.Printf("Ticker interval adjusted to %v", interval)
	}

	log.Println("Starting to monitor results")
	for {
		select {
		case result := <-r.ResultsCollectorChannel:
			r.processResult(result)
		case <-ticker.C:
			if r.allChecksComplete() {
				jsonResults, err := r.sendBatchedResults()
				if err == nil {
					// log.Println("Sending batched results to channel")
					r.ResultsChannel <- jsonResults
					if interval == 100*time.Millisecond {
						adjustTicker(30 * time.Second)
					}
				} else {
					// log.Println("Not all checks are complete yet")
				}
			}
		}
	}
}

func (r *IbpMonitor) processResult(resultJSON string) {
	// log.Printf("Processing result: %s", resultJSON)

	var tempResult map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &tempResult); err != nil {
		log.Printf("Error unmarshalling result: %v", err)
		return
	}

	serverName, ok := tempResult["ServerName"].(string)
	if !ok {
		log.Printf("Error: ServerName not found or not a string in result: %v", tempResult)
		return
	}

	checkName, ok := tempResult["CheckName"].(string)
	if !ok {
		log.Printf("Error: CheckName not found or not a string in result: %v", tempResult)
		return
	}

	// log.Printf("Result for server %s, check %s", serverName, checkName)

	r.mu.Lock()
	nodeResults, exists := r.NodeResults[serverName]
	if !exists {
		// log.Printf("Node results for server %s do not exist, creating new entry", serverName)
		nodeResults = &NodeResults{Checks: make(map[string]interface{})}
		r.NodeResults[serverName] = nodeResults
	} else {
		// log.Printf("Node results for server %s found", serverName)
	}
	r.mu.Unlock()

	nodeResults.mu.Lock()
	defer nodeResults.mu.Unlock()
	nodeResults.Checks[checkName] = tempResult
	// log.Printf("Check %s result updated for server %s", checkName, serverName)
}

func (r *IbpMonitor) sendBatchedResults() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	results := make(map[string]bool)
	for serverName, resultsStruct := range r.NodeResults {
		online := true
		for checkName, checkResult := range resultsStruct.Checks {
			checkResultMap, ok := checkResult.(map[string]interface{})
			if !ok {
				log.Printf("Error converting checkResult to map for server %s, check %s", serverName, checkName)
				continue
			}

			if success, exists := checkResultMap["Success"].(bool); exists && !success {
				online = false
				break
			}
		}
		results[serverName] = online
		// log.Printf("Server %s, Online: %t", serverName, online)
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		log.Printf("Error marshalling batched results: %v", err)
		return "", err
	}

	resultsString := string(resultsJSON)
	if resultsString == "{}" || resultsString == "" {
		// log.Println("Batched results are empty")
		return "", fmt.Errorf("JSON results are empty")
	}

	// log.Printf("Batched results: %s", resultsJSON)
	return string(resultsJSON), err
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

	for serverName, nodeResults := range r.NodeResults {
		for _, checkName := range checksToValidate {
			if _, exists := nodeResults.Checks[checkName]; !exists {
				log.Printf("Check %s not complete for server %s", checkName, serverName)
				return false
			}
		}
	}

	return true
}
