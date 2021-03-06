module github.com/temporalio/roadrunner-temporal

go 1.16

require (
	github.com/cenkalti/backoff/v4 v4.1.1
	github.com/fatih/color v1.12.0
	github.com/golang/protobuf v1.5.2
	github.com/json-iterator/go v1.1.11
	github.com/pborman/uuid v1.2.1
	// SPIRAL ========
	github.com/spiral/endure v1.0.2
	github.com/spiral/errors v1.0.11
	github.com/spiral/roadrunner/v2 v2.3.1
	// ===========
	github.com/stretchr/testify v1.7.0
	go.temporal.io/api v1.4.1-0.20210420220407-6f00f7f98373
	go.temporal.io/sdk v1.7.0
	go.uber.org/zap v1.17.0
	google.golang.org/protobuf v1.26.0
)
