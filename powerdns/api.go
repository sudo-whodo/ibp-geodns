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
	filteredConfigs := make([]DNS, len(powerDNSConfigs))
	for i, dns := range powerDNSConfigs {
		filteredConfigs[i].Domain = dns.Domain
		filteredConfigs[i].Members = make(map[string]Member)

		for memberName, member := range dns.Members {
			copiedMember := Member{
				MemberName: member.MemberName,
				IPv4:       member.IPv4,
				IPv6:       member.IPv6,
				Latitude:   member.Latitude,
				Longitude:  member.Longitude,
				Override:   member.Override,
				Results:    make(map[string]Result),
			}

			for checkKey, check := range member.Results {
				copiedMember.Results[checkKey] = check
			}

			filteredConfigs[i].Members[memberName] = copiedMember
		}
	}

	if req.Details != "" {
		memberName := req.Details
		for i := range filteredConfigs {
			if member, exists := filteredConfigs[i].Members[memberName]; exists {
				filteredConfigs[i].Members = map[string]Member{
					memberName: member,
				}
			} else {
				filteredConfigs[i].Members = map[string]Member{}
			}
		}
	}

	for i := range filteredConfigs {
		domain := filteredConfigs[i].Domain

		for memberName, member := range filteredConfigs[i].Members {
			filteredResults := make(map[string]Result)

			expectedPrefix := domain

			for checkKey, check := range member.Results {
				if strings.HasPrefix(checkKey, expectedPrefix) {
					filteredResults[checkKey] = check
				}
			}

			member.Results = filteredResults

			filteredConfigs[i].Members[memberName] = member
		}
	}

	response := Response{
		Result: filteredConfigs,
	}

	return response
}
