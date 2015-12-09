package ringpop

import (
	"fmt"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/ring"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim"
	"strings"
	"sync"
)

type statter struct {
	reporter bark.StatsReporter
	hostport string
	prefix   string
	keys     map[string]string
	mutex    sync.RWMutex
}

func NewStatter(address string, reporter bark.StatsReporter, emitters ...shared.EventEmitter) *statter {
	s := &statter{
		reporter: reporter,
		prefix:   toStatsPrefix(address),
		keys:     make(map[string]string),
	}

	for _, emitter := range emitters {
		emitter.RegisterListener(s.onEvent)
	}

	return s
}

func (s *statter) onEvent(event shared.Event) {
	switch event := event.(type) {
	case ring.RingChecksumEvent:
		s.reporter.IncCounter(s.key("ring.checksum-computed"), nil, 1)

	case ring.RingChangedEvent:
		added := int64(len(event.ServersAdded))
		removed := int64(len(event.ServersRemoved))
		s.reporter.IncCounter(s.key("ring.server-added"), nil, added)
		s.reporter.IncCounter(s.key("ring.server-removed"), nil, removed)

	case swim.MaxPAdjustedEvent:
		s.reporter.UpdateGauge(s.key("max-p"), nil, int64(event.NewPCount))

	case swim.JoinReceiveEvent:
		s.reporter.IncCounter(s.key("join.recv"), nil, 1)

	case swim.JoinCompleteEvent:
		s.reporter.IncCounter(s.key("join.complete"), nil, 1)
		s.reporter.RecordTimer(s.key("join"), nil, event.Duration)

	case swim.PingSendEvent:
		s.reporter.IncCounter(s.key("ping.send"), nil, 1)

	case swim.PingSendCompleteEvent:
		s.reporter.RecordTimer(s.key("ping"), nil, event.Duration)

	case swim.PingReceiveEvent:
		s.reporter.IncCounter(s.key("ping.recv"), nil, 1)

	case swim.PingRequestsSendEvent:
		s.reporter.IncCounter(s.key("ping-req.send"), nil, int64(len(event.Peers)))

	case swim.PingRequestsSendCompleteEvent:
		s.reporter.RecordTimer(s.key("ping-req"), nil, event.Duration)

	case swim.PingRequestReceiveEvent:
		s.reporter.IncCounter(s.key("ping-req.recv"), nil, 1)

	case swim.PingRequestPingEvent:
		s.reporter.RecordTimer(s.key("ping-req.ping"), nil, event.Duration)

	case swim.ProtocolDelayComputeEvent:
		s.reporter.RecordTimer(s.key("protocol.delay"), nil, event.Duration)

	case swim.ProtocolFrequencyEvent:
		s.reporter.RecordTimer(s.key("protocol.frequency"), nil, event.Duration)

	case swim.ChecksumComputeEvent:
		s.reporter.RecordTimer(s.key("compute-checksum"), nil, event.Duration)
		s.reporter.UpdateGauge(s.key("checksum"), nil, int64(event.Checksum))

	case swim.MemberlistChangesReceivedEvent:
		for _, change := range event.Changes {
			status := change.Status
			if len(status) == 0 {
				status = "unknown"
			}
			s.reporter.IncCounter(s.key("membership-update."+status), nil, 1)
		}
	}
}

func (s *statter) key(suffix string) string {
	s.mutex.RLock()
	key, ok := s.keys[suffix]
	s.mutex.RUnlock()

	if !ok {
		// Upgrade to RW, double-check.
		s.mutex.Lock()
		key, ok := s.keys[suffix]
		if !ok {
			key = s.prefix + suffix
			s.keys[key] = key
		}

		s.mutex.Unlock()
	}

	return key
}

// Transform address into Stats-compatible prefix.
// For example, from 192.168.0.12:3000 to 192_168_0_12_3000.
func toStatsPrefix(address string) (string) {
    prefix := strings.Replace(address, ".", "_", -1)
	prefix = strings.Replace(prefix, ":", "_", -1)
	prefix = fmt.Sprintf("ringpop.%s.", prefix)
    return prefix
}
