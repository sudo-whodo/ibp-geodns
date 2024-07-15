package ibpmonitor

import (
	"crypto/tls"
	"encoding/json"
	"ibp-geodns/config"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type WssResult struct {
	CheckName  string
	ServerName string
	CheckType  string
	Success    bool
	Latency    time.Duration
	Error      error
}

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

func WssCheck(member Member, config config.CheckConfig, resultsCollectorChannel chan string) {
	u, err := url.Parse(member.IPv4Address)
	if err != nil {
		log.Printf("Failed to parse WSS endpoint: %v\n", err)
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			ServerName: u.Hostname(),
		},
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.Dial(network, member.IPv4Address+":443")
		},
	}

	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("Failed to connect to WSS endpoint: %v\n", err)
	}
	defer c.Close()

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "chain_getBlockHash",
		Params:  []interface{}{"latest"},
		ID:      1,
	}

	if !sendJSONRPCRequest(c, request) {
		log.Printf("Failed to read message: %v\n", err)
	}

	_, _, err = c.ReadMessage()
	if err != nil {
		log.Printf("Failed to read message: %v\n", err)
	}

	//endTime := time.Now()
	//responseTime := endTime.Sub(startTime)
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

func init() {
	RegisterCheck("wss", WssCheck)
	RegisterResultType("wss", WssResult{})
}
