package aggregatedpool

import (
	"context"

	"github.com/temporalio/roadrunner-temporal/v6/api"
	"go.temporal.io/sdk/interceptor"
)

type workerInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func NewWorkerInterceptor() interceptor.WorkerInterceptor {
	return &workerInterceptor{}
}

func (*workerInterceptor) InterceptActivity(_ context.Context, next interceptor.ActivityInboundInterceptor) interceptor.ActivityInboundInterceptor {
	i := &activityInboundInterceptor{}
	i.Next = next
	return i
}

type activityInboundInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
}

func (a *activityInboundInterceptor) ExecuteActivity(ctx context.Context, in *interceptor.ExecuteActivityInput) (any, error) {
	// re-store headers under the RR context key
	ctx = context.WithValue(ctx, api.HeaderContextKey, interceptor.Header(ctx))
	return a.Next.ExecuteActivity(ctx, in)
}
