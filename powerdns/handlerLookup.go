package powerdns

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

func handleLookup(params Parameters) Response {
	domain := strings.ToLower(strings.TrimSuffix(params.Qname, "."))
	log.Printf("Looking up domain: %s, type: %s", domain, params.Qtype)

	// Check for ACME challenge records
	if strings.HasPrefix(domain, "_acme-challenge.") {
		acmeRecords, exists := staticEntries[domain]
		if exists && len(acmeRecords) > 0 {
			record := acmeRecords[0]
			if record.Qtype == "TXT" {
				acmeContent, err := fetchACMEChallenge(record.Content)
				if err == nil {
					return Response{Result: []Record{
						{
							Qtype:    "TXT",
							Qname:    domain,
							Content:  acmeContent,
							Ttl:      0,
							Auth:     true,
							DomainID: params.ZoneID,
						},
					}}
				} else {
					log.Printf("Failed to fetch ACME challenge content: %v", err)
				}
			}
		}
	}

	if records, exists := staticEntries[domain]; exists {
		staticRecords := []Record{}
		for _, record := range records {
			if record.Qtype == params.Qtype || params.Qtype == "ANY" {
				staticRecords = append(staticRecords, record)
			}
		}
		if len(staticRecords) > 0 {
			// log.Printf("Found static records for domain %s: %+v", domain, staticRecords)
			return Response{Result: staticRecords}
		}
	}

	records := []Record{}

	// Handle SOA queries
	if params.Qtype == "SOA" {
		parts := strings.Split(domain, ".")
		if len(parts) > 1 {
			topLevelDomain := strings.Join(parts[len(parts)-2:], ".")
			if topLevelDomains[topLevelDomain] {
				currentUnixTimestamp := int(time.Now().Unix())
				soaRecord := Record{
					Qtype:    "SOA",
					Qname:    topLevelDomain,
					Content:  fmt.Sprintf("dns-01.%s. hostmaster.%s. %d 3600 600 1209600 3600", topLevelDomain, topLevelDomain, currentUnixTimestamp),
					Ttl:      3600,
					Auth:     true,
					DomainID: params.ZoneID,
				}
				records = append(records, soaRecord)
				return Response{Result: records}
			}
		}
	}

	mu.RLock()
	defer mu.RUnlock()

	var closestMember Member
	minDistance := math.MaxFloat64

	clientIP := params.Remote
	clientLat, clientLon, err := getClientCoordinates(clientIP)
	if err != nil {
		log.Printf("Failed to get client coordinates for IP %s: %v", clientIP, err)
		return Response{Result: []Record{}}
	}

	for _, config := range powerDNSConfigs {
		if config.Domain == domain {
			for _, member := range config.Members {
				success := true
				for _, result := range member.Results {
					if !result.Success {
						success = false
						break
					}
				}
				if success {
					dist := distance(clientLat, clientLon, member.Latitude, member.Longitude)
					if dist < minDistance {
						minDistance = dist
						closestMember = member
					}
				}
			}

			if closestMember.MemberName != "" {
				if params.Qtype == "A" || params.Qtype == "ANY" {
					if closestMember.IPv4 != "" {
						records = append(records, Record{
							Qtype:    "A",
							Qname:    domain,
							Content:  closestMember.IPv4,
							Ttl:      30,
							Auth:     true,
							DomainID: params.ZoneID,
						})
					}
				}
				if params.Qtype == "AAAA" || params.Qtype == "ANY" {
					if closestMember.IPv6 != "" {
						records = append(records, Record{
							Qtype:    "AAAA",
							Qname:    domain,
							Content:  closestMember.IPv6,
							Ttl:      30,
							Auth:     true,
							DomainID: params.ZoneID,
						})
					}
				}
				break
			}
		}
	}

	if len(records) == 0 {
		for _, config := range powerDNSConfigs {
			if config.Domain == domain {
				if params.Qtype == "A" || params.Qtype == "ANY" {
					log.Printf("No records found for domain %s, returning default result", domain)
					defaultRecord := Record{
						Qtype:    "A",
						Qname:    domain,
						Content:  "192.96.202.175",
						Ttl:      30,
						Auth:     true,
						DomainID: params.ZoneID,
					}
					records = append(records, defaultRecord)
					break
				}
			}
		}
	}

	log.Printf("Found records: %+v", records)
	return Response{Result: records}
}

func fetchACMEChallenge(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch ACME challenge from %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := strings.TrimSpace(string(body))
	return content, nil
}
