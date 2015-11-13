package shared

import (
	"time"

	"golang.org/x/net/context"

	"github.com/uber/tchannel-go"
)

var retryOptions = &tchannel.RetryOptions{
	RetryOn: tchannel.RetryNever,
}

func NewTChannelContext(timeout time.Duration) (tchannel.ContextWithHeaders, context.CancelFunc) {
	return tchannel.NewContextBuilder(timeout).
		DisableTracing().
		SetRetryOptions(retryOptions).
		Build()
}
