module github.com/temporalio/roadrunner-temporal/v5

go 1.25.0

require (
	github.com/goccy/go-json v0.10.5
	github.com/google/uuid v1.6.0
	github.com/roadrunner-server/api/v4 v4.23.0
	github.com/roadrunner-server/endure/v2 v2.6.2
	github.com/roadrunner-server/errors v1.4.1
	github.com/roadrunner-server/events v1.0.1
	github.com/roadrunner-server/pool v1.1.3
	github.com/stretchr/testify v1.11.1
	github.com/uber-go/tally/v4 v4.1.17
	go.temporal.io/api v1.57.0
	go.temporal.io/sdk v1.37.0
	go.temporal.io/sdk/contrib/tally v0.2.0
	go.temporal.io/server v1.29.1
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
)

replace github.com/uber-go/tally/v4 => github.com/uber-go/tally/v4 v4.1.10

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cactus/go-statsd-client/v5 v5.1.0
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nexus-rpc/sdk-go v0.4.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.1 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/roadrunner-server/goridge/v3 v3.8.3
	github.com/robfig/cron v1.2.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.46.1-0.20251013234738-63d1a5100f82 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251103181224-f26f9409b101 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251103181224-f26f9409b101 // indirect
	google.golang.org/grpc v1.77.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
