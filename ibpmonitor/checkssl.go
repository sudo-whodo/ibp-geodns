package ibpmonitor

import (
	"crypto/tls"
	"encoding/json"
	"ibp-geodns/config"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

type SslResult struct {
	CheckName   string  `json:"checkname"`
	MemberName  string  `json:"membername"`
	EndpointURL string  `json:"endpointurl"`
	ResultType  string  `json:"resulttype"`
	Success     bool    `json:"success"`
	Error       string  `json:"error"`
	Data        SslData `json:"data"`
}

type SslData struct {
	ExpiryTimestamp int64 `json:"expirytimestamp"`
	DaysUntilExpiry int   `json:"daysuntilexpiry"`
}

func SslCheck(member Member, options config.CheckConfig, resultsCollectorChannel chan string) {

	var MaxConcurrentChecks = 20

	checkName := "ssl"
	connectTimeout := getIntOption(options.ExtraOptions, "ConnectTimeout", 4)
	uniqueHostnames := make(map[string]bool)

	for _, service := range member.Services {
		for _, endpoint := range service.Endpoints {
			endpointForParsing := strings.Replace(endpoint, "wss://", "https://", 1)

			u, err := url.Parse(endpointForParsing)
			if err != nil {
				//log.Printf("Error parsing endpoint '%s' for member %s: %v", endpoint, member.MemberName, err)
				continue
			}

			hostname := u.Hostname()
			if hostname != "" {
				//log.Printf("Extracted hostname '%s' from endpoint '%s'", hostname, endpoint)
				uniqueHostnames[hostname] = true
			}
		}
	}

	if len(uniqueHostnames) == 0 {
		//log.Printf("No valid endpoints found for member %s; skipping SSL check.", member.MemberName)
		return
	}

	var wg sync.WaitGroup
	semaphoreChan := make(chan struct{}, MaxConcurrentChecks)
	delayBetweenChecks := 1 * time.Millisecond

	for hostname := range uniqueHostnames {
		time.Sleep(delayBetweenChecks)

		wg.Add(1)
		go func(hostname string) {
			defer wg.Done()

			semaphoreChan <- struct{}{}
			defer func() { <-semaphoreChan }()

			ipAddress := member.IPv4Address
			tcpConn, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddress, "443"), time.Duration(connectTimeout)*time.Second)
			if err != nil {
				log.Printf("SSL check failed for member %s, Hostname %s: TCP Connection error", member.MemberName, hostname)

				result := SslResult{
					CheckName:   checkName,
					MemberName:  member.MemberName,
					EndpointURL: hostname,
					ResultType:  "endpoint",
					Success:     false,
					Error:       "TCP connection error",
					Data:        SslData{},
				}
				resultJSON, _ := json.Marshal(result)
				resultsCollectorChannel <- string(resultJSON)
				return
			}

			tlsConn := tls.Client(tcpConn, &tls.Config{
				ServerName:         hostname,
				InsecureSkipVerify: false,
			})

			err = tlsConn.Handshake()
			if err != nil {
				log.Printf("SSL check failed for member %s, Hostname %s: TLS handshake failed", member.MemberName, hostname)
				tlsConn.Close()

				result := SslResult{
					CheckName:   checkName,
					MemberName:  member.MemberName,
					EndpointURL: hostname,
					ResultType:  "endpoint",
					Success:     false,
					Error:       "TLS handshake failed",
					Data:        SslData{},
				}
				resultJSON, _ := json.Marshal(result)
				resultsCollectorChannel <- string(resultJSON)
				return
			}

			certs := tlsConn.ConnectionState().PeerCertificates

			cert := certs[0]
			expiryTimestamp := cert.NotAfter.Unix()
			daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)

			var success bool
			var errortext string

			if daysUntilExpiry < 5 {
				success = false
				errortext = "Less than 5 days until expiry"
			} else {
				success = true
				errortext = ""
			}

			if !success {
				log.Printf("SSL check failed for member %s, Hostname %s: Certificate expires in %d days", member.MemberName, hostname, daysUntilExpiry)
			}

			result := SslResult{
				CheckName:   checkName,
				MemberName:  member.MemberName,
				EndpointURL: hostname,
				ResultType:  "endpoint",
				Success:     success,
				Error:       errortext,
				Data: SslData{
					ExpiryTimestamp: expiryTimestamp,
					DaysUntilExpiry: daysUntilExpiry,
				},
			}
			resultJSON, _ := json.Marshal(result)
			resultsCollectorChannel <- string(resultJSON)

			tlsConn.Close()
		}(hostname)
	}

	wg.Wait()
}

func init() {
	RegisterCheck("ssl", SslCheck)
	RegisterResultType("ssl", SslResult{})
}
