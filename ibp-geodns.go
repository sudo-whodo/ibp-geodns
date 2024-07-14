package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"ibp-geodns/ibpconfig"
	"ibp-geodns/ibpmonitor"
	"ibp-geodns/powerdns"
)

type Config struct {
	GeoliteDBPath      string `json:"GeoliteDBPath"`
	StaticDNSConfigUrl string `json:"StaticDNSConfigUrl"`
	MembersConfigUrl   string `json:"MembersConfigUrl"`
	ServicesConfigUrl  string `json:"ServicesConfigUrl"`
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := &Config{}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func main() {
	log.Println("Starting the application...")

	config, err := loadConfig("config.json")
	if err != nil {
		log.Printf("Failed to load config: %v", err)
	}

	done := make(chan bool)
	ibpconfig.Init(done, config.MembersConfigUrl, config.ServicesConfigUrl)

	// Wait for the initial configuration to be ready
	log.Println("Waiting for initial configuration to be ready...")
	<-done
	log.Println("Initial configuration is ready")

	// Extract DNS and Endpoints
	log.Println("Extracting DNS and Endpoints...")
	endpoints, memberServices, serviceEndpoints := ibpconfig.ExtractData()
	log.Println("Extraction complete")

	// Populate PowerDNS configuration
	var powerDNSConfigs []powerdns.DNS
	for dns, members := range endpoints {
		dnsConfig := powerdns.DNS{
			Domain:  dns,
			Members: make(map[string]powerdns.Member),
		}
		for memberName, endpoint := range members {
			member := powerdns.Member{
				MemberName: memberName,
				IPv4:       endpoint.IPv4,
				IPv6:       endpoint.IPv6,
				Latitude:   endpoint.Latitude,
				Longitude:  endpoint.Longitude,
				Online:     false,
			}
			dnsConfig.Members[memberName] = member
		}
		powerDNSConfigs = append(powerDNSConfigs, dnsConfig)
	}
	log.Println("PowerDNS configuration populated")

	// Populate IBP Monitor configuration
	var ibpMonitorConfigs []ibpmonitor.Member
	for memberName, service := range memberServices {
		member := ibpmonitor.Member{
			MemberName:  memberName,
			IPv4Address: strings.Join(service.IPv4Addresses, ", "),
			IPv6Address: strings.Join(service.IPv6Addresses, ", "),
		}

		for _, serviceName := range service.Services {
			if serviceEndpoint, exists := serviceEndpoints[serviceName][memberName]; exists {
				endpoints := []string{}
				for _, url := range serviceEndpoint.URLs {
					endpoints = append(endpoints, url.URL)
				}
				service := ibpmonitor.Service{
					ServiceName: serviceName,
					Endpoints:   endpoints,
				}
				member.Services = append(member.Services, service)
			}
		}

		ibpMonitorConfigs = append(ibpMonitorConfigs, member)
	}
	log.Println("IBP Monitor configuration populated")

	// Setup IBP Monitor Health Checker
	options := ibpmonitor.Options{
		CheckInterval: 30 * time.Second,
		Timeout:       5 * time.Second,
		EnabledChecks: []string{"ping"},
	}
	healthChecker := ibpmonitor.NewRpcHealth(ibpMonitorConfigs, options)
	resultsChannel := healthChecker.Start()

	log.Println("Waiting for initial results to launch powerdns...")
	initialResultsJSON := <-resultsChannel
	log.Printf("Initial results received.")

	// Parse initial results
	initialResults := parseInitialResults(initialResultsJSON)

	// Initialize PowerDNS
	powerdns.Init(powerDNSConfigs, resultsChannel, initialResults, config.GeoliteDBPath, config.StaticDNSConfigUrl)

	select {} // Run forever
}

func parseInitialResults(initialResultsJSON string) map[string]bool {
	var initialResults map[string]bool
	if err := json.Unmarshal([]byte(initialResultsJSON), &initialResults); err != nil {
		log.Printf("Error parsing initial results: %v", err)
	}

	flatResults := make(map[string]bool)
	for memberName, success := range initialResults {
		flatResults[memberName] = success
	}

	return flatResults
}
