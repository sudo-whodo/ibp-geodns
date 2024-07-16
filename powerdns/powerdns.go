package powerdns

import (
	"ibp-geodns/config"
	"log"
	"net/http"
	"strings"
)

var (
	powerDNSConfigs []DNS
	resultsChannel  chan string
	configData      *config.Config
	staticEntries   map[string][]Record
	topLevelDomains map[string]bool
)

func Init(configs []DNS, resultsCh chan string, config *config.Config) {
	configData = config

	err := InitGeoIP(config.GeoliteDBPath)
	if err != nil {
		log.Printf("Failed to initialize GeoIP database: %v", err)
	}

	err = loadStaticEntries(config.StaticDNSConfigUrl)
	if err != nil {
		log.Printf("Failed to load static entries: %v", err)
	}

	go startStaticEntriesUpdater(config.StaticDNSConfigUrl)

	powerDNSConfigs = configs
	resultsChannel = resultsCh

	topLevelDomains = make(map[string]bool)
	for _, config := range configs {
		parts := strings.Split(config.Domain, ".")
		if len(parts) > 1 {
			topLevelDomain := strings.Join(parts[len(parts)-2:], ".")
			topLevelDomains[topLevelDomain] = true
		}
	}

	go updateMemberStatus()

	http.HandleFunc("/dns", dnsHandler)
	log.Println("Starting PowerDNS server on :8080")
	go http.ListenAndServe(":8080", nil)
}
