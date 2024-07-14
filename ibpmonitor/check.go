package ibpmonitor

import (
	"time"
)

var (
	checks = make(map[string]Check)
)

func RegisterCheck(name string, check Check) {
	resultTypesMutex.Lock()
	defer resultTypesMutex.Unlock()
	checks[name] = check
}

func GetCheck(name string) (Check, bool) {
	resultTypesMutex.Lock()
	defer resultTypesMutex.Unlock()
	check, exists := checks[name]
	return check, exists
}

func (r *RpcHealth) isCheckEnabled(name string) bool {
	if len(r.options.EnabledChecks) == 0 {
		// If no specific checks are enabled, all checks are considered enabled
		return true
	}
	for _, enabledCheck := range r.options.EnabledChecks {
		if enabledCheck == name {
			return true
		}
	}
	return false
}

func (r *RpcHealth) performChecks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Members) == 0 {
		// No members to check
		return
	}

	for _, member := range r.Members {
		for name, check := range checks {
			if r.isCheckEnabled(name) {
				if member.IPv4Address != "" {
					go check(RpcServer{Name: member.MemberName, RpcUrl: "", Options: RpcServerOptions{IpAddress: member.IPv4Address}}, r.options, r.ResultsCollectorChannel)
					time.Sleep(100 * time.Millisecond)
				}

				if member.IPv6Address != "" {
					go check(RpcServer{Name: member.MemberName, RpcUrl: "", Options: RpcServerOptions{IpAddress: member.IPv6Address}}, r.options, r.ResultsCollectorChannel)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}
}
