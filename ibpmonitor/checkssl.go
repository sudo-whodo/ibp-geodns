package ibpmonitor

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type SslResult struct {
	CheckName       string
	ServerName      string
	Success         bool
	ExpiryTimestamp int64
	DaysUntilExpiry int
	Valid           bool
	Error           string
}

func SslCheck(server RpcServer, options Options, resultsCollectorChannel chan string) {
	checkName := "ssl"
	done := make(chan SslResult, 2)
	timer := time.NewTimer(options.Timeout)

	go func() {
		// Recover from any fatal errors in the check.
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Sprintf("Ssl check failed: %v", r)
				done <- SslResult{CheckName: checkName, ServerName: server.Name, Success: false, Error: err}
			}
			close(done)
		}()

		u, err := url.Parse(server.RpcUrl)
		if err != nil {
			err := fmt.Sprintf("Unable to parse wss endpoint: %v", err)
			done <- SslResult{CheckName: checkName, ServerName: server.Name, Success: false, Error: err}
			return
		}
		hostname := u.Hostname()

		conn, err := tls.Dial("tcp", server.Options.IpAddress+":443", &tls.Config{
			ServerName:         hostname,
			InsecureSkipVerify: true,
		})
		if err != nil {
			err := fmt.Sprintf("Failed to connect to endpoint: %v", err)
			done <- SslResult{CheckName: checkName, ServerName: server.Name, Success: false, Error: err}
			return
		}
		defer conn.Close()

		var isRpcUrlValid bool
		var certExpired bool
		var expiryTimestamp int64
		var daysUntilExpiry int
		var success bool

		// Iterate through each certificate
		for _, cert := range conn.ConnectionState().PeerCertificates {
			for _, domain := range cert.DNSNames {
				if strings.HasPrefix(domain, "*.") {
					rootDomain := strings.TrimPrefix(domain, "*.")
					if strings.HasSuffix(hostname, rootDomain) && strings.Count(hostname, ".") == strings.Count(rootDomain, ".")+1 {
						isRpcUrlValid = true
					}
				} else if domain == hostname {
					isRpcUrlValid = true
				}
			}

			if isRpcUrlValid {
				expiryTimestamp = cert.NotAfter.Unix()
				daysUntilExpiry = int(time.Until(cert.NotAfter).Hours() / 24)
				certExpired = time.Now().After(cert.NotAfter)
				break
			}
		}

		if !certExpired && isRpcUrlValid {
			success = true
		}

		result := SslResult{
			CheckName:       checkName,
			ServerName:      server.Name,
			Success:         success,
			ExpiryTimestamp: expiryTimestamp,
			DaysUntilExpiry: daysUntilExpiry,
		}

		done <- result
	}()

	select {
	case result := <-done:
		resultJSON, _ := json.Marshal(result)
		resultsCollectorChannel <- string(resultJSON)
	case <-timer.C:
		err := fmt.Sprintf("sslCheck for %s timed out", server.Name)
		resultJSON, _ := json.Marshal(SslResult{CheckName: checkName, ServerName: server.Name, Success: false, Error: err})
		resultsCollectorChannel <- string(resultJSON)
	}
}

func init() {
	RegisterCheck("ssl", SslCheck)
	RegisterResultType("ssl", SslResult{})
}
