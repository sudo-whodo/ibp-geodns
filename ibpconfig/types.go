package ibpconfig

type Member struct {
	Details struct {
		Name    string `json:"Name"`
		Website string `json:"Website"`
		Logo    string `json:"Logo"`
	} `json:"Details"`
	Membership struct {
		MemberLevel int `json:"MemberLevel"`
		Joined      int `json:"Joined"`
		LastRankup  int `json:"LastRankup"`
	} `json:"Membership"`
	Service struct {
		Active      int    `json:"Active"`
		ServiceIPv4 string `json:"ServiceIPv4"`
		ServiceIPv6 string `json:"ServiceIPv6"`
		MonitorUrl  string `json:"MonitorUrl"`
	} `json:"Service"`
	ServiceAssignments map[string][]string `json:"ServiceAssignments"`
	Location           struct {
		Region    string  `json:"Region"`
		Latitude  float64 `json:"Latitude"`
		Longitude float64 `json:"Longitude"`
	} `json:"Location"`
}

type Service struct {
	Configuration struct {
		ServiceType   string `json:"ServiceType"`
		Active        int    `json:"Active"`
		LevelRequired int    `json:"LevelRequired"`
		NetworkName   string `json:"NetworkName"`
	} `json:"Configuration"`
	Providers map[string]struct {
		RpcUrls []string `json:"RpcUrls"`
	} `json:"Providers"`
}

type Endpoint struct {
	MemberName   string
	IPv4         string
	IPv6         string
	Latitude     float64
	Longitude    float64
	OriginalURLs []OriginalURL
}

type OriginalURL struct {
	URL         string
	NetworkName string
}

type MemberService struct {
	IPv4Addresses []string
	IPv6Addresses []string
	Services      []string
}

type ServiceEndpoint struct {
	URLs         []OriginalURL
	ServiceIPv4s []string
	ServiceIPv6s []string
	Domains      []string
}
