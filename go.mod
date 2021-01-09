module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fatih/color v1.10.0
	github.com/golang/protobuf v1.4.3
	github.com/json-iterator/go v1.1.10
	github.com/pborman/uuid v1.2.1
	github.com/spiral/endure v1.0.0-beta20
	github.com/spiral/errors v1.0.6
	github.com/spiral/roadrunner/v2 v2.0.0-beta4
	github.com/stretchr/testify v1.6.1
	github.com/vmihailenco/msgpack/v5 v5.1.4
	go.temporal.io/api v1.4.0
	go.temporal.io/sdk v1.2.0
	go.uber.org/zap v1.16.0
)

replace go.temporal.io/sdk v1.2.0 => ../sdk-go

replace github.com/spiral/roadrunner/v2 v2.0.0-beta4 => ../roadrunner
