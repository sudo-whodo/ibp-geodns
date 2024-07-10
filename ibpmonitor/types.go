package ibpmonitor

import (
	"sync"
	"time"
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

type Check func(rpcServer RpcServer, options Options, resultsCollectorChannel chan string)

type Options struct {
	CheckInterval time.Duration
	Timeout       time.Duration
	Continuous    bool
	EnabledChecks []string
}

type NodeResults struct {
	mu     sync.Mutex
	Checks map[string]interface{}
}

type RpcHealth struct {
	mu                      sync.Mutex
	Members                 []Member
	HealthStatus            map[string]bool
	StopChannel             chan struct{}
	options                 Options
	ResultsChannel          chan string
	ResultsCollectorChannel chan string
	NodeResults             map[string]*NodeResults
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
