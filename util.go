package ringpop

import (
	"strings"
	"time"

	"github.com/uber/bark"
)

type noopStatsReporter struct{}

func (noopStatsReporter) IncCounter(name string, tags bark.Tags, value int64)      {}
func (noopStatsReporter) UpdateGauge(name string, tags bark.Tags, value int64)     {}
func (noopStatsReporter) RecordTimer(name string, tags bark.Tags, d time.Duration) {}

func genStatsHostport(hostport string) string {
	return strings.Replace(strings.Replace(hostport, ".", "_", -1), ":", "_", -1)
}
