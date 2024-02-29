package helpers

import (
	"context"
	"os"

	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
)

type TemporalInterceptorPlugin struct {
	config Configurer
}

func (pt *TemporalInterceptorPlugin) Init(cfg Configurer) error {
	pt.config = cfg
	return nil
}

func (pt *TemporalInterceptorPlugin) Serve() chan error {
	errCh := make(chan error, 1)
	return errCh
}

func (pt *TemporalInterceptorPlugin) Stop(context.Context) error {
	return nil
}

func (pt *TemporalInterceptorPlugin) Name() string {
	return "temporal_test.incterceptor_plugin"
}

func (pt *TemporalInterceptorPlugin) WorkerInterceptor() interceptor.WorkerInterceptor {
	return &workerInterceptor{}
}

type workerInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func (w *workerInterceptor) InterceptActivity(
	_ context.Context,
	next interceptor.ActivityInboundInterceptor,
) interceptor.ActivityInboundInterceptor {
	f, err := os.Create("./interceptor_test")
	if err != nil {
		panic(err)
	}
	_ = f.Close()
	return next
}

func (w *workerInterceptor) InterceptWorkflow(
	_ workflow.Context,
	next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	return next
}
