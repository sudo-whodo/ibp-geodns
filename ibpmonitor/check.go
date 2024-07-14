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

func (r *IbpMonitor) performCheck(checkName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Members) == 0 {
		// No members to check
		return
	}

	for _, member := range r.Members {
		if check, exists := checks[checkName]; exists {
			go CheckWrapper(checkName, check, member, r.Config.Checks[checkName], r.ResultsCollectorChannel)
			time.Sleep(100 * time.Microsecond)
		}
	}
}
