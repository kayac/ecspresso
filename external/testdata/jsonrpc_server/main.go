// Command jsonrpc_server is a JSON-RPC 2.0 server used by the external
// plugin tests. It is built on the creachadair/jrpc2 library so the tests
// exercise interoperability between ecspresso's hand-written client and a
// standard JSON-RPC 2.0 server. It lives in its own Go module so the main
// ecspresso module does not depend on jrpc2.
package main

import (
	"context"
	"flag"
	"os"
	"sync/atomic"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
)

var (
	delay   = flag.Duration("delay", 0, "delay before each response")
	respErr = flag.Bool("error", false, "reply with a jsonrpc error")
)

func main() {
	flag.Parse()

	var count atomic.Int64
	myfunc := func(ctx context.Context, params []string) (map[string]any, error) {
		n := count.Add(1)
		if *delay > 0 {
			time.Sleep(*delay)
		}
		if *respErr {
			return nil, jrpc2.Errorf(jrpc2.Code(-32000), "boom")
		}
		result := map[string]any{"count": n}
		if len(params) > 0 {
			result["param"] = params[0]
		}
		return result, nil
	}

	srv := jrpc2.NewServer(handler.Map{
		"myfunc": handler.New(myfunc),
	}, nil)
	srv.Start(channel.Line(os.Stdin, os.Stdout))
	srv.Wait()
}
