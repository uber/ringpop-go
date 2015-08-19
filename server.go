package ringpop

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/json"
	"golang.org/x/net/context"
)

type headers map[string]string

type arg struct{}

type server struct {
	ringpop *Ringpop
	channel interface{}
}

func newServer(ringpop *Ringpop) (*server, error) {
	if ringpop.channel == nil {
		return nil, errors.New("ringpop channel cannot be nil")
	}

	s := &server{
		ringpop: ringpop,
		channel: ringpop.channel,
	}

	var handlers = map[string]interface{}{
		"/admin/debugSet":    s.debugSetHandler,
		"/admin/debugClear":  s.debugClearHandler,
		"/admin/stats":       s.statsHandler,
		"/protocol/join":     s.joinHandler,
		"/protocol/ping":     s.pingHandler,
		"/protocol/ping-req": s.pingReqHandler,
		"/forward/req":       s.forwardReqHandler,
	}

	// register handlers
	err := json.Register(s.channel.(tchannel.Registrar), handlers, func(ctx context.Context, err error) {
		s.ringpop.logger.WithField("error", err).Info("[ringpop] error occured")
	})
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *server) listenAndServe() error {
	// listen on the socket for the top-level tchannel
	// subchannel is expected to have a top channel which is listening
	switch s.ringpop.channel.(type) {
	case *tchannel.Channel:
		return s.ringpop.channel.(*tchannel.Channel).ListenAndServe(s.ringpop.WhoAmI())
	}
	return nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PROTOCOL HANDLERS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (s *server) joinHandler(ctx json.Context, reqBody *joinBody) (*joinResBody, error) {
	resBody, err := handleJoin(s.ringpop, *reqBody)
	if err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":   s.ringpop.WhoAmI(),
			"error":   err,
			"handler": "join-recv",
		}).Debug("[ringpop] join receive failed")
		return nil, err
	}

	return &resBody, nil
}

func (s *server) pingHandler(ctx json.Context, reqBody *pingBody) (*pingBody, error) {
	resBody := handlePing(s.ringpop, *reqBody)
	return &resBody, nil
}

func (s *server) pingReqHandler(ctx json.Context, reqBody *pingReqBody) (*pingReqRes, error) {
	resBody := handlePingReq(s.ringpop, *reqBody)
	return &resBody, nil
}

func (s *server) forwardReqHandler(ctx json.Context, req *forwardReq) (*forwardReqRes, error) {
	resBody, err := handleForwardRequest(s.ringpop, &req.Header, req)
	return &resBody, err
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// ADMIN HANDLERS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (s *server) statsHandler(ctx json.Context, arg *arg) (map[string]interface{}, error) {
	resBody := handleStats(s.ringpop)
	return resBody, nil
}

func (s *server) debugSetHandler(ctx json.Context, arg *arg) (res *arg, err error) {
	s.ringpop.logger.Level = log.DebugLevel
	return
}

func (s *server) debugClearHandler(ctx json.Context, arg *arg) (res *arg, err error) {
	s.ringpop.logger.Level = log.InfoLevel
	return
}
