module github.com/temporalio/roadrunner-temporal/v6

go 1.27

require (
	github.com/cactus/go-statsd-client/v5 v5.1.0
	github.com/goccy/go-json v0.11.2
	github.com/google/uuid v1.6.0
	github.com/prometheus/client_golang v1.25.0
	github.com/roadrunner-server/api-go/v6 v6.0.0-beta.15
	github.com/roadrunner-server/api-plugins/v6 v6.0.0-beta.2
	github.com/roadrunner-server/endure/v2 v2.6.2
	github.com/roadrunner-server/errors v1.5.0
	github.com/roadrunner-server/events v1.0.1
	github.com/roadrunner-server/goridge/v4 v4.0.0-beta.3
	github.com/roadrunner-server/pool/v2 v2.0.0-beta.1
	github.com/stretchr/testify v1.12.1
	github.com/uber-go/tally/v4 v4.1.17
	go.temporal.io/api v1.63.6
	go.temporal.io/sdk v1.49.0
	go.temporal.io/sdk/contrib/sysinfo v0.1.1
	go.temporal.io/sdk/contrib/tally v0.2.0
	go.temporal.io/server v1.32.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.22.0 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/log v0.2.0 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20260802145828-341c2f0c90b5 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nexus-rpc/nexus-proto-annotations v0.1.0 // indirect
	github.com/nexus-rpc/sdk-go v0.7.0 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20260916203055-22a1a467d9f0 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	// gopsutil v4.25+ is required: it adds cgo-free darwin CPU sampling, which SysInfoProvider
	// heartbeat metrics need in CGO_ENABLED=0 builds. Do not let this fall back to the
	// contrib/sysinfo minimum (v4.24.8).
	github.com/shirou/gopsutil/v4 v4.26.9 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/twmb/murmur3 v1.2.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261005182115-fad411399dd8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261005182115-fad411399dd8 // indirect
)
