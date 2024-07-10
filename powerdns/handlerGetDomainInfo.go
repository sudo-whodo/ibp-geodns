package powerdns

import "time"

func handleGetDomainInfo(params Parameters) Response {
	currentUnixTimestamp := int(time.Now().Unix())
	for _, config := range powerDNSConfigs {
		if config.Domain == params.Qname {
			return Response{Result: DomainInfo{
				ID:             0,
				Zone:           config.Domain,
				Masters:        []string{},
				NotifiedSerial: currentUnixTimestamp,
				Serial:         currentUnixTimestamp,
				LastCheck:      currentUnixTimestamp,
				Kind:           "NATIVE",
			}}
		}
	}
	return Response{Result: nil}
}
