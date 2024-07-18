package powerdns

import (
	"encoding/json"
	"net/http"
	"sort"
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
		res = status()
	default:
		http.Error(w, "Method not supported", http.StatusNotImplemented)
		return
	}

	//log.Printf("Sending response: %+v\n", res)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func enableMember(req ApiRequest) Response {
	if authValue, ok := configData.AuthKey[req.Details]; !ok || req.AuthKey != authValue {
		return Response{
			Result: "Unauthorized access",
		}
	} else if authValue, ok := configData.AuthKey["root"]; !ok || req.AuthKey != authValue {
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
	if authValue, ok := configData.AuthKey[req.Details]; !ok || req.AuthKey != authValue {
		return Response{
			Result: "Unauthorized access",
		}
	} else if authValue, ok := configData.AuthKey["root"]; !ok || req.AuthKey != authValue {
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

func status() Response {
	sort.SliceStable(powerDNSConfigs, func(i, j int) bool {
		return powerDNSConfigs[i].Domain < powerDNSConfigs[j].Domain
	})

	for i := range powerDNSConfigs {
		memberNames := make([]string, 0, len(powerDNSConfigs[i].Members))
		for memberName := range powerDNSConfigs[i].Members {
			memberNames = append(memberNames, memberName)
		}
		sort.Strings(memberNames)

		sortedMembers := make(map[string]Member)
		for _, memberName := range memberNames {
			sortedMembers[memberName] = powerDNSConfigs[i].Members[memberName]
		}
		powerDNSConfigs[i].Members = sortedMembers
	}

	response := Response{
		Result: powerDNSConfigs,
	}

	return response
}
