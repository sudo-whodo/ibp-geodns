package powerdns

import "time"

type Record struct {
	Qtype    string `json:"qtype"`
	Qname    string `json:"qname"`
	Content  string `json:"content"`
	Ttl      int    `json:"ttl"`
	Auth     bool   `json:"auth"`
	DomainID int    `json:"domain_id"`
}

type DNS struct {
	Domain  string            `json:"domain"`
	Members map[string]Member `json:"members"`
}

type Member struct {
	MemberName string            `json:"member_name"`
	IPv4       string            `json:"ipv4"`
	IPv6       string            `json:"ipv6"`
	Latitude   float64           `json:"latitude"`
	Longitude  float64           `json:"longitude"`
	Override   bool              `json:"override"`
	Results    map[string]Result `json:"results"`
}

type Result struct {
	Success   bool      `json:"success"`
	Data      string    `json:"checkError"`
	OfflineTS time.Time `json:"offline_ts,omitempty"`
}

type ApiRequest struct {
	Method  string `json:"method"`
	Details string `json:"details"`
	AuthKey string `json:"authkey"`
}

type Request struct {
	Method     string     `json:"method"`
	Parameters Parameters `json:"parameters"`
}

type Response struct {
	Result interface{} `json:"result"`
}

type Parameters struct {
	Local      string `json:"local"`
	Qname      string `json:"qname"`
	Qtype      string `json:"qtype"`
	RealRemote string `json:"real-remote"`
	Remote     string `json:"remote"`
	ZoneID     int    `json:"zone-id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Key        struct {
		ID        int    `json:"id"`
		Flags     int    `json:"flags"`
		Active    bool   `json:"active"`
		Published bool   `json:"published"`
		Content   string `json:"content"`
	} `json:"key"`
}

type DomainInfo struct {
	ID             int      `json:"id"`
	Zone           string   `json:"zone"`
	Masters        []string `json:"masters"`
	NotifiedSerial int      `json:"notified_serial"`
	Serial         int      `json:"serial"`
	LastCheck      int      `json:"last_check"`
	Kind           string   `json:"kind"`
}
