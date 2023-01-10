package aggregatedpool

import (
	"context"

	"github.com/temporalio/roadrunner-temporal/v3/common"
	"go.temporal.io/sdk/interceptor"
)

type workerInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func NewWorkerInterceptor() interceptor.WorkerInterceptor {
	return &workerInterceptor{}
}

func (w *workerInterceptor) InterceptActivity(_ context.Context, next interceptor.ActivityInboundInterceptor) interceptor.ActivityInboundInterceptor {
	i := &activityInboundInterceptor{root: w}
	i.Next = next
	return i
}

type activityInboundInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
	root *workerInterceptor
}

func (a *activityInboundInterceptor) Init(outbound interceptor.ActivityOutboundInterceptor) error {
	i := &activityOutboundInterceptor{root: a.root}
	i.Next = outbound
	return a.Next.Init(i)
}

func (a *activityInboundInterceptor) ExecuteActivity(ctx context.Context, in *interceptor.ExecuteActivityInput) (any, error) {
	// re-store headers under the RR context key
	ctx = context.WithValue(ctx, common.HeaderContextKey, interceptor.Header(ctx))
	return a.Next.ExecuteActivity(ctx, in)
}

type activityOutboundInterceptor struct {
	interceptor.ActivityOutboundInterceptorBase
	root *workerInterceptor
}
