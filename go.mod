module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/fatih/color v1.7.0
	github.com/json-iterator/go v1.1.10
	github.com/spf13/cobra v1.1.0
	github.com/spiral/endure v1.0.0-beta9
	github.com/spiral/roadrunner/v2 v2.0.0-alpha12
	go.temporal.io/api v1.0.0
	go.temporal.io/sdk v1.1.0
	go.uber.org/zap v1.16.0
)

replace github.com/spiral/roadrunner/v2 v2.0.0-alpha12 => ./roadrunner
