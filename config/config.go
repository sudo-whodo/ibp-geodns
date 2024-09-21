package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

var (
	MembersURL  string
	ServicesURL string
	Members     map[string]Member
	Services    map[string]Service
)

func fetchAndValidateJSON(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to fetch data")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return err
	}

	return nil
}

func updateConfigurations(done chan bool) {
	log.Println("Fetching and updating configurations...")

	var newMembers map[string]Member
	if err := fetchAndValidateJSON(MembersURL, &newMembers); err != nil {
		log.Printf("Error fetching members configuration: %v", err)
	} else {
		Members = newMembers
		log.Println("Updated members configuration.")
	}

	var newServices map[string]Service
	if err := fetchAndValidateJSON(ServicesURL, &newServices); err != nil {
		log.Printf("Error fetching services configuration: %v", err)
	} else {
		Services = newServices
		log.Println("Updated services configuration.")
	}

	if done != nil {
		done <- true
		close(done)
		done = nil
	}

}

func ExtractData() (map[string]map[string]Endpoint, map[string]MemberService, map[string]map[string]ServiceEndpoint) {
	endpoints := make(map[string]map[string]Endpoint)
	memberServices := make(map[string]MemberService)
	serviceEndpoints := make(map[string]map[string]ServiceEndpoint)

	for memberName, member := range Members {
		if member.Service.Active != 1 {
			continue
		}

		memberService := memberServices[memberName]
		memberService.IPv4Addresses = appendUniqueString(memberService.IPv4Addresses, member.Service.ServiceIPv4)
		memberService.IPv6Addresses = appendUniqueString(memberService.IPv6Addresses, member.Service.ServiceIPv6)

		for _, services := range member.ServiceAssignments {
			for _, service := range services {
				if serviceConfig, exists := Services[service]; exists {
					if serviceConfig.Configuration.Active == 1 && member.Membership.MemberLevel >= serviceConfig.Configuration.LevelRequired {
						memberService.Services = appendUniqueString(memberService.Services, service)

						for _, providerData := range serviceConfig.Providers {
							for _, url := range providerData.RpcUrls {
								dnsName := extractDNSName(url)
								if dnsName != "" {
									if endpoints[dnsName] == nil {
										endpoints[dnsName] = make(map[string]Endpoint)
									}
									originalURL := OriginalURL{
										URL:         url,
										NetworkName: serviceConfig.Configuration.NetworkName,
									}
									endpoint := Endpoint{
										MemberName:      memberName,
										ExpectedNetwork: serviceConfig.Configuration.NetworkName,
										IPv4:            member.Service.ServiceIPv4,
										IPv6:            member.Service.ServiceIPv6,
										Latitude:        member.Location.Latitude,
										Longitude:       member.Location.Longitude,
										OriginalURLs:    []OriginalURL{originalURL},
									}
									if existing, exists := endpoints[dnsName][memberName]; exists {
										endpoint.OriginalURLs = append(existing.OriginalURLs, originalURL)
									}
									endpoints[dnsName][memberName] = endpoint

									if serviceEndpoints[service] == nil {
										serviceEndpoints[service] = make(map[string]ServiceEndpoint)
									}
									serviceEndpoint := serviceEndpoints[service][memberName]
									serviceEndpoint.ExpectedNetwork = serviceConfig.Configuration.NetworkName
									serviceEndpoint.URLs = append(serviceEndpoint.URLs, originalURL)
									serviceEndpoint.ServiceIPv4s = appendUniqueString(serviceEndpoint.ServiceIPv4s, member.Service.ServiceIPv4)
									serviceEndpoint.ServiceIPv6s = appendUniqueString(serviceEndpoint.ServiceIPv6s, member.Service.ServiceIPv6)
									serviceEndpoint.Domains = appendUniqueString(serviceEndpoint.Domains, dnsName)
									serviceEndpoints[service][memberName] = serviceEndpoint
								}
							}
						}
					}
				}
			}
		}
		memberServices[memberName] = memberService
	}

	return endpoints, memberServices, serviceEndpoints
}

func extractDNSName(url string) string {
	if strings.HasPrefix(url, "wss://") || strings.HasPrefix(url, "https://") {
		return strings.Split(strings.TrimPrefix(strings.TrimPrefix(url, "wss://"), "https://"), "/")[0]
	}
	return ""
}

func appendUniqueString(slice []string, item string) []string {
	if item == "" {
		return slice
	}
	for _, elem := range slice {
		if elem == item {
			return slice
		}
	}
	return append(slice, item)
}

func Init(done chan bool, membersURL, servicesURL string) {
	MembersURL = membersURL
	ServicesURL = servicesURL
	go updateConfigurations(done)
}
