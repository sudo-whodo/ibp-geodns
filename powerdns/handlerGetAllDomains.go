package powerdns

import "time"

func handleGetAllDomains() Response {

	mu.RLock()
	defer mu.RUnlock()

	currentUnixTimestamp := int(time.Now().Unix())
	domains := []DomainInfo{}
	for domain := range topLevelDomains {
		domains = append(domains, DomainInfo{
			ID:             0,
			Zone:           domain,
			Masters:        []string{},
			NotifiedSerial: currentUnixTimestamp,
			Serial:         currentUnixTimestamp,
			LastCheck:      currentUnixTimestamp,
			Kind:           "NATIVE",
		})
	}

	return Response{Result: domains}
}
