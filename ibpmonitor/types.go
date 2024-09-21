package ibpmonitor

import (
	"ibp-geodns/config"
	"sync"
)

type RpcServerOptions struct {
	IpAddress     string
	ResolveRpcUrl bool
	Network       string
}

type RpcServer struct {
	Name    string
	RpcUrl  string
	Options RpcServerOptions
}

type Check func(member Member, options config.CheckConfig, resultsCollectorChannel chan string)

type NodeResults struct {
	Checks         map[string]interface{}            // For site-wide checks
	EndpointChecks map[string]map[string]interface{} // For endpoint-specific checks
	mu             sync.Mutex
}

type IbpMonitor struct {
	mu                      sync.Mutex
	Members                 []Member
	HealthStatus            map[string]bool
	StopChannel             chan struct{}
	Config                  *config.Config
	ResultsChannel          chan string
	ResultsCollectorChannel chan string
	NodeResults             map[string]*NodeResults
	InitialResultsProcessed bool
}

type Service struct {
	ServiceName string   `json:"service_name"`
	Endpoints   []string `json:"endpoints"`
}

type Member struct {
	MemberName  string    `json:"member_name"`
	IPv4Address string    `json:"ipv4_address"`
	IPv6Address string    `json:"ipv6_address"`
	Services    []Service `json:"services"`
}
