package main

import (
	"encoding/json"
	"ibp-geodns/config"
	"ibp-geodns/ibpmonitor"
	"ibp-geodns/powerdns"
	"log"
	"os"
	"strings"
)

func loadConfig(filename string) (*config.Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := &config.Config{}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func main() {
	log.Println("Starting the application...")

	configfile, err := loadConfig("config.json")
	if err != nil {
		log.Printf("Failed to load config: %v", err)
	}

	done := make(chan bool)
	config.Init(done, configfile.MembersConfigUrl, configfile.ServicesConfigUrl)

	log.Println("Waiting for initial configuration to be ready...")
	<-done
	log.Println("Initial configuration is ready")

	log.Println("Extracting DNS and Endpoints...")
	endpoints, memberServices, serviceEndpoints := config.ExtractData()
	log.Println("Extraction complete")

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
				Results:    make(map[string]powerdns.Result),
			}
			dnsConfig.Members[memberName] = member
		}
		powerDNSConfigs = append(powerDNSConfigs, dnsConfig)
	}
	log.Println("PowerDNS configuration populated")

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
					ServiceName: serviceEndpoint.ExpectedNetwork,
					Endpoints:   endpoints,
				}
				member.Services = append(member.Services, service)
			}
		}

		ibpMonitorConfigs = append(ibpMonitorConfigs, member)
	}
	log.Println("IBP Monitor configuration populated")

	healthChecker := ibpmonitor.NewIbpMonitor(ibpMonitorConfigs, configfile)
	resultsChannel := healthChecker.Start()

	powerdns.Init(powerDNSConfigs, resultsChannel, configfile)

	select {}
}
