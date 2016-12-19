// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ringpop

import (
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/uber/tchannel-go"
)

func handleStats(rp *Ringpop) map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	servers := rp.ring.Servers()

	type stats map[string]interface{}

	uptime, _ := rp.Uptime()

	return stats{
		"hooks":      nil,
		"membership": rp.node.MemberStats(),
		"process": stats{
			"memory": stats{
				"rss":       memStats.Sys,
				"heapTotal": memStats.HeapSys,
				"heapUsed":  memStats.HeapInuse,
			},
			"pid": os.Getpid(),
		},
		"protocol": rp.node.ProtocolStats(),
		"ring": stats{
			"servers":   servers,
			"checksum":  rp.ring.Checksum(),
			"checksums": rp.ring.Checksums(),
		},
		"version":         "???", // TODO: version!
		"timestamp":       time.Now().Unix(),
		"uptime":          uptime,
		"tchannelVersion": strconv.Itoa(tchannel.CurrentProtocolVersion), // get proper version
	}
}
