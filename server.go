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
	channel *tchannel.Channel
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
		"/protocol/join":     s.joinHandler,
		"/protocol/ping":     s.pingHandler,
		"/protocol/ping-req": s.pingReqHandler,
		"/proxy/req":         s.proxyReqHandler,
	}

	// register handlers
	err := json.Register(s.channel, handlers, func(ctx context.Context, err error) {
		s.ringpop.logger.WithField("error", err).Info("[ringpop] error occured")
	})
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *server) listenAndServe() error {
	return s.channel.ListenAndServe(s.ringpop.WhoAmI())
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

func (s *server) proxyReqHandler(ctx json.Context, req *proxyReq) (*proxyReqRes, error) {
	resBody := handleProxyRequest(s.ringpop, ctx.Headers().(*proxyReqHeader), req)
	return &resBody, nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// ADMIN HANDLERS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (s *server) debugSetHandler(ctx json.Context, arg *arg) (res *arg, err error) {
	s.ringpop.logger.Level = log.DebugLevel
	return
}

func (s *server) debugClearHandler(ctx json.Context, arg *arg) (res *arg, err error) {
	s.ringpop.logger.Level = log.InfoLevel
	return
}
