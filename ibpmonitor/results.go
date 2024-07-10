package ibpmonitor

import (
	"encoding/json"
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

func (r *RpcHealth) MonitorResults() {
	for {
		select {
		case result := <-r.ResultsCollectorChannel:
			r.processResult(result)
		case <-time.After(5 * time.Second):
			if r.allChecksComplete() {
				jsonResults := r.sendBatchedResults()
				if jsonResults != "{}" {
					r.ResultsChannel <- jsonResults
				}
			} else {
				log.Println("Not all checks are complete yet")
			}
		}
	}
}

func (r *RpcHealth) processResult(resultJSON string) {
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

	r.mu.Lock()
	nodeResults, exists := r.NodeResults[serverName]
	if !exists {
		nodeResults = &NodeResults{Checks: make(map[string]interface{})}
		r.NodeResults[serverName] = nodeResults
	}
	r.mu.Unlock()

	nodeResults.mu.Lock()
	defer nodeResults.mu.Unlock()
	nodeResults.Checks[checkName] = tempResult
}

func (r *RpcHealth) sendBatchedResults() string {
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
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		log.Printf("Error marshalling batched results: %v", err)
		return ""
	}

	return string(resultsJSON)
}

func (r *RpcHealth) allChecksComplete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	var checksToValidate []string
	if len(r.options.EnabledChecks) > 0 {
		checksToValidate = r.options.EnabledChecks
	} else {
		for checkName := range checks {
			checksToValidate = append(checksToValidate, checkName)
		}
	}

	for serverName, nodeResults := range r.NodeResults {
		for _, checkName := range checksToValidate {
			if _, exists := nodeResults.Checks[checkName]; !exists {
				log.Printf("Check %s not complete for server %s", checkName, serverName) // Log incomplete check
				return false
			}
		}
	}
	return true
}
