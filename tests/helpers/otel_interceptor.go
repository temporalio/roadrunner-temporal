package helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdkinterceptor "go.temporal.io/sdk/interceptor"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelinterceptor "go.temporal.io/sdk/contrib/opentelemetry"
)

// InMemoryOtelInterceptorPlugin implements api.Interceptor using an in-memory
// OpenTelemetry tracer provider. This allows tests to assert on captured spans
// without needing an external collector or stderr capture.
type InMemoryOtelInterceptorPlugin struct {
	tp  *sdktrace.TracerProvider
	Exp *tracetest.InMemoryExporter
	wi  sdkinterceptor.WorkerInterceptor
}

// NewInMemoryOtelInterceptorPlugin creates a new in-memory OTEL interceptor plugin
// for testing. The exporter is accessible via the Exp field.
func NewInMemoryOtelInterceptorPlugin(t *testing.T) *InMemoryOtelInterceptorPlugin {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shut down tracer provider: %v", err)
		}
	})

	tracer := tp.Tracer("temporal-test")
	ti, err := otelinterceptor.NewTracingInterceptor(otelinterceptor.TracerOptions{Tracer: tracer})
	require.NoError(t, err)

	return &InMemoryOtelInterceptorPlugin{tp: tp, Exp: exp, wi: ti}
}

func (i *InMemoryOtelInterceptorPlugin) Init(_ Configurer) error {
	return nil
}

func (i *InMemoryOtelInterceptorPlugin) Serve() chan error {
	return make(chan error, 1)
}

func (i *InMemoryOtelInterceptorPlugin) Stop(context.Context) error {
	return nil
}

func (i *InMemoryOtelInterceptorPlugin) Name() string {
	return "in_memory_otel_interceptor"
}

func (i *InMemoryOtelInterceptorPlugin) WorkerInterceptor() sdkinterceptor.WorkerInterceptor {
	return i.wi
}
