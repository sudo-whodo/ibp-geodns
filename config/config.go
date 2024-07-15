package config

type CheckConfig struct {
	Enabled       int                    `json:"Enabled"`
	CheckType     string                 `json:"CheckType"`
	Timeout       int                    `json:"Timeout"`
	CheckInterval int                    `json:"CheckInterval"`
	ExtraOptions  map[string]interface{} `json:"ExtraOptions"`
}

type Config struct {
	GeoliteDBPath      string                 `json:"GeoliteDBPath"`
	StaticDNSConfigUrl string                 `json:"StaticDNSConfigUrl"`
	MembersConfigUrl   string                 `json:"MembersConfigUrl"`
	ServicesConfigUrl  string                 `json:"ServicesConfigUrl"`
	Checks             map[string]CheckConfig `json:"Checks"`
}

type SiteCheckResult struct {
	CheckName  string                 `json:"checkname"`
	Success    bool                   `json:"success"`
	CheckError string                 `json:"checkerror,omitempty"`
	CheckData  map[string]interface{} `json:"checkdata,omitempty"`
}

type EndpointCheckResult struct {
	CheckName  string                 `json:"checkname"`
	Success    bool                   `json:"success"`
	CheckError string                 `json:"checkerror,omitempty"`
	CheckData  map[string]interface{} `json:"checkdata,omitempty"`
}

type SiteResults struct {
	ResultType string                                `json:"resulttype"`
	Members    map[string]map[string]SiteCheckResult `json:"members"`
}

type EndpointResults struct {
	ResultType string                                               `json:"resulttype"`
	Endpoint   map[string]map[string]map[string]EndpointCheckResult `json:"endpoint"`
}
