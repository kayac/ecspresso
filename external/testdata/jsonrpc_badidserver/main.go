// Command jsonrpc_badidserver is a deliberately non-conforming JSON-RPC
// server used to test the client's response-id validation: it replies with
// an id that does not match the request. It has no dependencies and stays
// in the main ecspresso module (a correct jrpc2 server cannot reproduce an
// id mismatch).
package main

import (
	"bufio"
	"encoding/json"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		enc.Encode(map[string]any{ //nolint:errcheck
			"jsonrpc": "2.0",
			"id":      req.ID + 1000, // intentionally mismatched
			"result":  map[string]any{"count": 1},
		})
	}
}
