module github.com/temporalio/roadrunner-temporal

go 1.16

require (
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/fatih/color v1.12.0
	github.com/golang/protobuf v1.5.2
	github.com/json-iterator/go v1.1.11
	github.com/pborman/uuid v1.2.1
	// SPIRAL ========
	github.com/spiral/endure v1.0.1
	github.com/spiral/errors v1.0.9
	github.com/spiral/roadrunner/v2 v2.2.0
	// ===========
	github.com/stretchr/testify v1.7.0
	go.temporal.io/api v1.4.1-0.20210318194442-3f93fcec559f
	go.temporal.io/sdk v1.6.0
	go.uber.org/zap v1.17.0
	google.golang.org/protobuf v1.26.0
)
