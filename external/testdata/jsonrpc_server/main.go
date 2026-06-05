package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type request struct {
	JSONRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	ID      int64    `json:"id"`
}

type response struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	ID      int64  `json:"id"`
}

var (
	delay   = flag.Duration("delay", 0, "delay before each response")
	badID   = flag.Bool("bad-id", false, "reply with a mismatched response id")
	respErr = flag.Bool("error", false, "reply with a jsonrpc error")
)

func main() {
	flag.Parse()
	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var count int
	for scanner.Scan() {
		count++
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			fmt.Fprintf(os.Stderr, "jsonrpc_server: decode error: %v\n", err)
			continue
		}
		if *delay > 0 {
			time.Sleep(*delay)
		}
		resp := response{JSONRPC: "2.0", ID: req.ID}
		switch {
		case *respErr:
			resp.Error = map[string]any{"code": -32000, "message": "boom"}
		case *badID:
			resp.ID = req.ID + 1000
			resp.Result = map[string]any{"count": count}
		default:
			result := map[string]any{"count": count}
			if len(req.Params) > 0 {
				result["param"] = req.Params[0]
			}
			resp.Result = result
		}
		if err := enc.Encode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "jsonrpc_server: encode error: %v\n", err)
		}
	}
}
