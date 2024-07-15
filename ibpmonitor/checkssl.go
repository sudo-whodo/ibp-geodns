package ibpmonitor

import (
	"crypto/tls"
	"fmt"
	"ibp-geodns/config"
	"net/url"
	"strings"
	"time"
)

type SslResult struct {
	CheckName       string
	ServerName      string
	CheckType       string
	Success         bool
	ExpiryTimestamp int64
	DaysUntilExpiry int
	Valid           bool
	Error           string
}

func SslCheck(member Member, config config.CheckConfig, resultsCollectorChannel chan string) {
	checkName := "ssl"
	done := make(chan SslResult, 2)

	u, err := url.Parse(member.IPv4Address)
	if err != nil {
		err := fmt.Sprintf("Unable to parse wss endpoint: %v", err)
		done <- SslResult{CheckName: checkName, ServerName: member.MemberName, Success: false, Error: err}
		return
	}
	hostname := u.Hostname()

	conn, err := tls.Dial("tcp", member.IPv4Address+":443", &tls.Config{
		ServerName:         hostname,
		InsecureSkipVerify: true,
	})
	if err != nil {
		err := fmt.Sprintf("Failed to connect to endpoint: %v", err)
		done <- SslResult{CheckName: checkName, ServerName: member.MemberName, Success: false, Error: err}
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
		ServerName:      member.MemberName,
		Success:         success,
		ExpiryTimestamp: expiryTimestamp,
		DaysUntilExpiry: daysUntilExpiry,
	}

	done <- result
}

func init() {
	RegisterCheck("ssl", SslCheck)
	RegisterResultType("ssl", SslResult{})
}
