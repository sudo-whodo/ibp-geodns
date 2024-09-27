package powerdns

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

func apiHandler(w http.ResponseWriter, r *http.Request) {
	var req ApiRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var res Response

	switch req.Method {
	case "enableMember":
		res = enableMember(req)
	case "disableMember":
		res = disableMember(req)
	case "listMembers":
		res = listMembers()
	case "status":
		res = status(req)
	default:
		http.Error(w, "Method not supported", http.StatusNotImplemented)
		return
	}

	// log.Printf("Sending response: %+v\n", res)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func enableMember(req ApiRequest) Response {
	authValue := configData.AuthKey[req.Details]
	rootAuthValue := configData.AuthKey["root"]

	if (req.AuthKey != authValue) && (req.AuthKey != rootAuthValue) {
		return Response{
			Result: "Unauthorized access",
		}
	}

	memberName := req.Details
	success := 0

	for i := range powerDNSConfigs {
		for name, member := range powerDNSConfigs[i].Members {
			if name == memberName {
				member.Override = false
				powerDNSConfigs[i].Members[name] = member
				success = 1
			}
		}
	}

	response := Response{
		Result: success,
	}

	return response
}

func disableMember(req ApiRequest) Response {
	authValue := configData.AuthKey[req.Details]
	rootAuthValue := configData.AuthKey["root"]

	if (req.AuthKey != authValue) && (req.AuthKey != rootAuthValue) {
		return Response{
			Result: "Unauthorized access",
		}
	}

	memberName := req.Details
	success := 0

	for i := range powerDNSConfigs {
		for name, member := range powerDNSConfigs[i].Members {
			if name == memberName {
				member.Override = true
				powerDNSConfigs[i].Members[name] = member
				success = 1
			}
		}
	}

	response := Response{
		Result: success,
	}

	return response
}

func listMembers() Response {
	uniqueMembersMap := make(map[string]Member)

	for _, dnsConfig := range powerDNSConfigs {
		for memberName, member := range dnsConfig.Members {
			uniqueMembersMap[memberName] = member
		}
	}

	uniqueMembers := make([]Member, 0, len(uniqueMembersMap))
	for _, member := range uniqueMembersMap {
		uniqueMembers = append(uniqueMembers, member)
	}
	sort.Slice(uniqueMembers, func(i, j int) bool {
		return uniqueMembers[i].MemberName < uniqueMembers[j].MemberName
	})

	response := Response{
		Result: uniqueMembers,
	}

	return response
}

func status(req ApiRequest) Response {
	// Step 1: Make a deep copy of powerDNSConfigs
	filteredConfigs := make([]DNS, len(powerDNSConfigs))
	for i, dns := range powerDNSConfigs {
		// Copy the DNS struct
		filteredConfigs[i].Domain = dns.Domain
		filteredConfigs[i].Members = make(map[string]Member)

		// Copy each member
		for memberName, member := range dns.Members {
			// Create a copy of the Member struct
			copiedMember := Member{
				MemberName: member.MemberName,
				IPv4:       member.IPv4,
				IPv6:       member.IPv6,
				Latitude:   member.Latitude,
				Longitude:  member.Longitude,
				Override:   member.Override,
				Results:    make(map[string]Result),
			}

			// Copy the Results map
			for checkKey, check := range member.Results {
				copiedMember.Results[checkKey] = check
			}

			// Assign the copied member to the filteredConfigs
			filteredConfigs[i].Members[memberName] = copiedMember
		}
	}

	// Step 2: Check if req.Details contains the member name
	if req.Details != "" {
		memberName := req.Details
		for i := range filteredConfigs {
			// Check if the member exists in this domain
			if member, exists := filteredConfigs[i].Members[memberName]; exists {
				// Retain only the specified member
				filteredConfigs[i].Members = map[string]Member{
					memberName: member,
				}
			} else {
				// Member not found in this domain; retain no members
				filteredConfigs[i].Members = map[string]Member{}
			}
		}
	}

	// Step 3: For each member in filteredConfigs, ensure only relevant checks are included
	for i := range filteredConfigs {
		domain := filteredConfigs[i].Domain

		for memberName, member := range filteredConfigs[i].Members {
			// Create a new Results map
			filteredResults := make(map[string]Result)

			// Define the expected prefix for checks belonging to the current domain
			expectedPrefix := domain + "::"

			// Iterate over each check in the member's Results
			for checkKey, check := range member.Results {
				if strings.HasPrefix(checkKey, expectedPrefix) || !strings.Contains(checkKey, "::") {
					// Include checks specific to the domain or global site checks
					filteredResults[checkKey] = check
				}
				// Else, exclude the check as it belongs to a different domain
			}

			// Update the member's Results with the filtered checks
			member.Results = filteredResults

			// Assign the modified member back to the map
			mu.Lock()
			filteredConfigs[i].Members[memberName] = member
			mu.Unlock()
		}
	}

	// Step 4: Prepare the response with the filteredConfigs
	response := Response{
		Result: filteredConfigs,
	}

	return response
}
