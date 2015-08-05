package ringpop

import (
	"os"
	"runtime"
	"sort"
	"time"
)

type memberSorter []*Member

func (s *memberSorter) Len() int           { return len(*s) }
func (s *memberSorter) Swap(i, j int)      { (*s)[i], (*s)[j] = (*s)[j], (*s)[i] }
func (s *memberSorter) Less(i, j int) bool { return (*s)[i].Address < (*s)[j].Address }

func handleStats(rp *Ringpop) map[string]interface{} {
	rp.stat("increment", "stats", 1)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// hist := rp.gossip.protocolTiming
	servers := make([]string, 0, len(rp.ring.servers))
	for k := range rp.ring.servers {
		servers = append(servers, k)
	}

	members := memberSorter(rp.membership.memberlist)
	sort.Sort(&members)

	type smap map[string]interface{}
	return smap{
		// "hooks": nil,
		"membership": smap{
			"checksum": rp.membership.checksum,
			"members":  members,
		},
		"process": smap{
			"memory": smap{
				"rss":       memStats.Sys,
				"heapTotal": memStats.HeapSys,
				"heapUsed":  memStats.HeapInuse,
			},
			"pid": os.Getpid(),
		},
		// "protocol": smap{
		// 	"timing": smap{
		// 		"type":     "histogram",
		// 		"min":      hist.Min(),
		// 		"max":      hist.Max(),
		// 		"sum":      hist.Sum(),
		// 		"variance": hist.Variance(),
		// 		"mean":     hist.Mean(),
		// 		"std_dev":  hist.StdDev(),
		// 		"count":    hist.Count(),
		// 		"median":   hist.Percentile(0.5),
		// 		"p75":      hist.Percentile(0.75),
		// 		"p95":      hist.Percentile(0.95),
		// 		"p99":      hist.Percentile(0.99),
		// 		"p999":     hist.Percentile(0.999),
		// 	},
		// 	"protocolRate": rp.gossip.computeProtocolRate(),
		// 	"clientRate":   rp.clientRate.Rate1(),
		// 	"serverRate":   rp.serverRate.Rate1(),
		// 	"totalRate":    rp.totalRate.Rate1(),
		// },
		"ring": servers,
		// TODO (wieger) get version of go ringpop
		// "version": "v?",
		// TODO (wieger) get proper tchannel-go version of go ringpop
		// "tchannelversion": strconv.Itoa(tchannel.CurrentProtocolVersion),
		"timestamp": time.Now().UnixNano() / 1e6,
		"uptime":    time.Now().Unix() - rp.startTime.Unix(),
	}
}
