package shared

import (
	"time"

	"github.com/uber/tchannel-go"

	"golang.org/x/net/context"
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
