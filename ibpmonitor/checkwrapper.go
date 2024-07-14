package ibpmonitor

import (
	"encoding/json"
	"fmt"
	"ibp-geodns/config"
	"time"
)

func CheckWrapper(checkName string, checkFunc Check, member Member, options config.CheckConfig, resultsCollectorChannel chan string) {
	done := make(chan interface{}, 2)
	timer := time.NewTimer(time.Duration(options.Timeout) * time.Second)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Sprintf("%s check failed: %v", checkName, r)
				done <- map[string]interface{}{"CheckName": checkName, "ServerName": member.MemberName, "Success": false, "Error": err}
			}
			close(done)
		}()

		checkFunc(member, options, resultsCollectorChannel)
	}()

	select {
	case result := <-done:
		if resMap, ok := result.(map[string]interface{}); ok {
			resultJSON, _ := json.Marshal(resMap)
			resultsCollectorChannel <- string(resultJSON)
		}
	case <-timer.C:
		err := fmt.Sprintf("%s check for %s timed out", checkName, member.MemberName)
		resultJSON, _ := json.Marshal(map[string]interface{}{"CheckName": checkName, "ServerName": member.MemberName, "Success": false, "Error": err})
		resultsCollectorChannel <- string(resultJSON)
	}
}
