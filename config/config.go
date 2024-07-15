package config

type CheckConfig struct {
	Enabled       int                    `json:"Enabled"`
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
