package powerdns

import (
	"encoding/json"
	"net/http"
)

func dnsHandler(w http.ResponseWriter, r *http.Request) {
	var req Request
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// log.Printf("Received request: %+v\n", req)

	var res Response

	switch req.Method {
	case "lookup":
		res = handleLookup(req.Parameters)
	case "getDomainInfo":
		res = handleGetDomainInfo(req.Parameters)
	case "getAllDomains":
		res = handleGetAllDomains()
	case "getDomainKeys":
		res = handleGetDomainKeys(req.Parameters)
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
