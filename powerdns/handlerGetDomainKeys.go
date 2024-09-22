package powerdns

import "strings"

func handleGetDomainKeys(params Parameters) Response {

	mu.RLock()
	defer mu.RUnlock()

	for _, config := range powerDNSConfigs {
		normalizedDomain := strings.TrimSuffix(config.Domain, ".")
		if normalizedDomain == strings.TrimSuffix(params.Qname, ".") {
			keys := []struct {
				ID        int    `json:"id"`
				Flags     int    `json:"flags"`
				Active    bool   `json:"active"`
				Published bool   `json:"published"`
				Content   string `json:"content"`
			}{{
				ID:        3,
				Flags:     257,
				Active:    true,
				Published: true,
				Content:   config.Domain + " IN DNSKEY 257 3 13 Ts7EglQbnyZDVklFGoiAnbB/DGzlJC4RBft7/wouiSxgQ9OB7sXD9yOkhyjhs5BzaOFs0LivpUwQZnYFkafAYA==",
			}}

			return Response{Result: keys}
		}
	}

	return Response{Result: nil}
}
