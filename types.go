package main

type CheckConfig struct {
	Timeout       int `json:"timeout"`
	CheckInterval int `json:"checkInterval"`
}

type Config struct {
	GeoliteDBPath      string                 `json:"GeoliteDBPath"`
	StaticDNSConfigUrl string                 `json:"StaticDNSConfigUrl"`
	MembersConfigUrl   string                 `json:"MembersConfigUrl"`
	ServicesConfigUrl  string                 `json:"ServicesConfigUrl"`
	Checks             map[string]CheckConfig `json:"Checks"`
}
