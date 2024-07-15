package powerdns

import (
	"encoding/json"
	"log"
)

func updateMemberStatus() {
	for result := range resultsChannel {

		var status map[string]bool
		if err := json.Unmarshal([]byte(result), &status); err != nil {
			log.Printf("Error parsing result: %v", err)
			continue
		}

		// logResults(status)

		mu.Lock()
		for memberName, success := range status {
			for i, config := range powerDNSConfigs {
				if member, exists := config.Members[memberName]; exists {
					member.Online = success
					powerDNSConfigs[i].Members[memberName] = member
				}
			}
		}
		mu.Unlock()
	}
}

func logResults(status map[string]bool) {
	log.Println("Received member status update:")
	for memberName, success := range status {
		log.Printf("Member: %s, Online: %t\n", memberName, success)
	}
}
