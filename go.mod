module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/buger/goterm v0.0.0-20200322175922-2f3e71b85129
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.10.0
	github.com/json-iterator/go v1.1.10
	github.com/mattn/go-runewidth v0.0.9
	github.com/olekukonko/tablewriter v0.0.4
	github.com/pborman/uuid v1.2.1
	github.com/spf13/cobra v1.1.0
	github.com/spiral/endure v1.0.0-beta20
	github.com/spiral/errors v1.0.6
	github.com/spiral/goridge/v2 v2.4.6
	github.com/spiral/roadrunner/v2 v2.0.0-alpha25
	github.com/stretchr/testify v1.6.1
	github.com/vbauerster/mpb/v5 v5.3.0
	go.temporal.io/sdk v1.1.0
	go.uber.org/zap v1.16.0
)

replace go.temporal.io/sdk v1.1.0 => /home/valery/Projects/temp/go-sdk
