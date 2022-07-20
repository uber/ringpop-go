module github.com/uber/ringpop-go

go 1.18

require (
	github.com/benbjohnson/clock v0.0.0-20160125162948-a620c1cc9866
	github.com/cactus/go-statsd-client/statsd v0.0.0-20190922033735-5ca90424ceb7
	github.com/dgryski/go-farm v0.0.0-20140601200337-fc41e106ee0e
	github.com/rcrowley/go-metrics v0.0.0-20141108142129-dee209f2455f
	github.com/sirupsen/logrus v1.0.2-0.20170726183946-abee6f9b0679
	github.com/stretchr/testify v1.7.0
	github.com/uber-common/bark v1.0.0
	github.com/uber/tchannel-go v1.32.0
	github.com/vektra/mockery v0.0.0-20160406211542-130a05e1b51a
	golang.org/x/net v0.0.0-20220121210141-e204ce36a2ba
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/uber-go/atomic => go.uber.org/atomic v1.7.0

replace github.com/codegangsta/cli => github.com/urfave/cli v1.22.9
