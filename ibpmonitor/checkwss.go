package ibpmonitor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"ibp-geodns/config"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/semaphore"
)

type WssResult struct {
	CheckName   string      `json:"checkname"`
	ServerName  string      `json:"membername"`
	EndpointURL string      `json:"endpointurl"`
	ResultType  string      `json:"resulttype"`
	Success     bool        `json:"success"`
	Latency     float64     `json:"latency"`
	Error       string      `json:"error"`
	Data        interface{} `json:"data,omitempty"`
}

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

func WssCheck(member Member, options config.CheckConfig, resultsCollectorChannel chan string) {

	var MaxConcurrentChecks = 20
	connectTimeout := getIntOption(options.ExtraOptions, "ConnectTimeout", 4)

	sem := semaphore.NewWeighted(int64(MaxConcurrentChecks))

	var wg sync.WaitGroup

	for _, service := range member.Services {
		for _, endpoint := range service.Endpoints {
			if err := sem.Acquire(context.Background(), 1); err != nil {
				log.Printf("Failed to acquire semaphore: %v", err)
				continue
			}

			wg.Add(1)
			go func(service Service, endpoint string) {
				defer sem.Release(1)
				defer wg.Done()

				u, err := url.Parse(endpoint)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to parse WSS endpoint '%s': %v", endpoint, err)
					sendResult("wss", member.MemberName, "invalid-hostname", "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				hostname := u.Hostname()
				reconstructedURL := fmt.Sprintf("wss://%s%s", hostname, u.Path)

				dialer := websocket.Dialer{
					TLSClientConfig: &tls.Config{
						ServerName:         hostname,
						InsecureSkipVerify: false,
					},
					NetDial: func(network, addr string) (net.Conn, error) {

						return net.DialTimeout(network, net.JoinHostPort(member.IPv4Address, "443"), time.Duration(connectTimeout)*time.Second)
					},
					HandshakeTimeout: time.Duration(connectTimeout) * time.Second,
				}

				c, _, err := dialer.Dial(reconstructedURL, nil)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to connect to WSS endpoint (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				defer func() {
					if err := c.Close(); err != nil {
						log.Printf("Failed to close connection to (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					}
				}()

				request := JSONRPCRequest{
					JSONRPC: "2.0",
					Method:  "chain_getBlockHash",
					Params:  []interface{}{"latest"},
					ID:      1,
				}

				if !sendJSONRPCRequest(c, request) {
					errMsg := fmt.Sprintf("Failed to send JSON-RPC request to (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				_, _, err = c.ReadMessage()
				if err != nil {
					errMsg := fmt.Sprintf("Failed to read JSON-RPC response from (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				isFullArchive, err := checkFullArchive(c)
				if err != nil {
					errMsg := fmt.Sprintf("Full archive check failed for (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}
				if !isFullArchive {
					errMsg := fmt.Sprintf("Endpoint is not a full archive node (Member: %s URL: '%s')", member.MemberName, endpoint)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				isCorrectNetwork, err := checkNetwork(c, service.ServiceName)
				if err != nil {
					errMsg := fmt.Sprintf("Network check failed for (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}
				if !isCorrectNetwork {
					errMsg := fmt.Sprintf("Endpoint is not on the expected network (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				hasEnoughPeers, isSyncing, err := checkPeers(c)
				if err != nil {
					errMsg := fmt.Sprintf("Peers check failed for (Member: %s URL: '%s' Error: %v)", member.MemberName, endpoint, err)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}
				if !hasEnoughPeers || isSyncing {
					errMsg := fmt.Sprintf("Endpoint has insufficient peers or is syncing (Member: %s URL: '%s')", member.MemberName, endpoint)
					sendResult("wss", member.MemberName, hostname, "endpoint", false, errMsg, resultsCollectorChannel)
					log.Println(errMsg)
					return
				}

				sendResult("wss", member.MemberName, hostname, "endpoint", true, "", resultsCollectorChannel)
			}(service, endpoint)
		}
	}

	wg.Wait()
}

func sendJSONRPCRequest(c *websocket.Conn, request JSONRPCRequest) bool {
	requestBytes, err := json.Marshal(request)
	if err != nil {
		log.Printf("Failed to marshal JSON RPC request: %v\n", err)
		return false
	}

	if err := c.WriteMessage(websocket.TextMessage, requestBytes); err != nil {
		log.Printf("Failed to write message: %v\n", err)
		return false
	}

	return true
}

func checkFullArchive(c *websocket.Conn) (bool, error) {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "chain_getBlockHash",
		Params:  []interface{}{0},
		ID:      2,
	}

	if !sendJSONRPCRequest(c, request) {
		return false, fmt.Errorf("failed to send chain_getBlockHash request")
	}

	_, message, err := c.ReadMessage()
	if err != nil {
		return false, fmt.Errorf("failed to read chain_getBlockHash response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		return false, fmt.Errorf("failed to unmarshal chain_getBlockHash response: %v", err)
	}

	result, ok := response["result"].(string)
	if !ok || result == "" {
		return false, fmt.Errorf("chain_getBlockHash result is invalid")
	}

	return true, nil
}

func checkNetwork(c *websocket.Conn, expectedNetwork string) (bool, error) {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "system_chain",
		Params:  []interface{}{},
		ID:      3,
	}

	if !sendJSONRPCRequest(c, request) {
		return false, fmt.Errorf("failed to send system_chain request")
	}

	_, message, err := c.ReadMessage()
	if err != nil {
		return false, fmt.Errorf("failed to read system_chain response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		return false, fmt.Errorf("failed to unmarshal system_chain response: %v", err)
	}

	chain, ok := response["result"].(string)
	if !ok || chain == "" {
		return false, fmt.Errorf("system_chain result is invalid")
	}

	if !strings.EqualFold(chain, expectedNetwork) {
		return false, fmt.Errorf("node is on '%s' network instead of expected '%s'", chain, expectedNetwork)
	}

	return true, nil
}

func checkPeers(c *websocket.Conn) (bool, bool, error) {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "system_health",
		Params:  []interface{}{},
		ID:      4,
	}

	if !sendJSONRPCRequest(c, request) {
		return false, false, fmt.Errorf("failed to send system_health request")
	}

	_, message, err := c.ReadMessage()
	if err != nil {
		return false, false, fmt.Errorf("failed to read system_health response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		return false, false, fmt.Errorf("failed to unmarshal system_health response: %v", err)
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return false, false, fmt.Errorf("system_health result is invalid")
	}

	peers, ok := result["peers"].(float64)
	if !ok {
		return false, false, fmt.Errorf("peers count is invalid")
	}

	isSyncing, ok := result["isSyncing"].(bool)
	if !ok {
		return false, false, fmt.Errorf("isSyncing status is invalid")
	}

	hasEnoughPeers := peers > 5
	return hasEnoughPeers, isSyncing, nil
}

func init() {
	RegisterCheck("wss", WssCheck)
	RegisterResultType("wss", WssResult{})
}
