package rrtemporal

import (
	"context"
	"os"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/pool"
	"github.com/temporalio/roadrunner-temporal/v5/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v5/dataconverter"
	"github.com/temporalio/roadrunner-temporal/v5/internal/codec/proto"
	"github.com/temporalio/roadrunner-temporal/v5/internal/logger"
	tclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
)

const (
	APIKey string = "ApiKey"
)

func (p *Plugin) initPool() error {
	var err error
	var options []staticPool.Options

	if p.config.DisableActivityWorkers {
		options = append(options, staticPool.WithNumWorkers(0))
	}

	ap, err := p.server.NewPoolWithOptions(context.Background(), p.config.Activities, map[string]string{RrMode: pluginName, RrCodec: RrCodecVal}, p.log, options...)
	if err != nil {
		return err
	}

	dc := dataconverter.NewDataConverter(converter.GetDefaultDataConverter())
	codec := proto.NewCodec(p.log, dc)

	// LA + A definitions
	actDef := aggregatedpool.NewActivityDefinition(codec, ap, p.log, p.config.DisableActivityWorkers)
	laDef := aggregatedpool.NewLocalActivityFn(codec, ap, p.log)
	// ------------------

	// ---------- WORKFLOW POOL -------------
	wp, err := p.server.NewPool(
		context.Background(),
		&pool.Config{
			NumWorkers:      1,
			Command:         p.config.Activities.Command,
			AllocateTimeout: time.Hour * 240,
			// use the same timeout
			DestroyTimeout: p.config.Activities.DestroyTimeout,
			// no supervisor for the workflow worker
			Supervisor: nil,
		},
		map[string]string{RrMode: pluginName, RrCodec: RrCodecVal},
		nil,
	)
	if err != nil {
		return err
	}

	if len(wp.Workers()) < 1 {
		return errors.E(errors.Str("failed to allocate a workflow worker"))
	}

	// set all fields
	// we have only 1 worker for the workflow pool
	p.wwPID = int(wp.Workers()[0].Pid())

	wfDef := aggregatedpool.NewWorkflowDefinition(codec, laDef.ExecuteLA, wp, p.log)

	// get worker information
	wi, err := WorkerInfo(codec, wp, p.rrVersion, p.wwPID)
	if err != nil {
		return err
	}

	if len(wi) == 0 {
		return errors.Str("worker info should contain at least 1 worker")
	}

	err = p.initTemporalClient(wi[0].PhpSdkVersion, wi[0].Flags, dc)
	if err != nil {
		return err
	}

	workers, err := aggregatedpool.TemporalWorkers(wfDef, actDef, wi, p.log, p.temporal.client, p.temporal.interceptors)
	if err != nil {
		return err
	}

	for i := range workers {
		err = workers[i].Start()
		if err != nil {
			return err
		}
	}

	p.temporal.rrWorkflowDef = wfDef
	p.temporal.rrActivityDef = actDef
	p.temporal.workers = workers
	p.codec = codec

	p.temporal.activities = ActivitiesInfo(wi)
	p.temporal.workflows = WorkflowsInfo(wi)
	p.actP = ap
	p.wfP = wp

	return nil
}

func (p *Plugin) getWfDef() *aggregatedpool.Workflow {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.temporal.rrWorkflowDef
}

func (p *Plugin) getActDef() *aggregatedpool.Activity {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.temporal.rrActivityDef
}

func (p *Plugin) initTemporalClient(phpSdkVersion string, flags map[string]string, dc converter.DataConverter) error {
	if phpSdkVersion == "" {
		phpSdkVersion = clientBaselineVersion
	}

	if val, ok := flags[APIKey]; ok {
		if val != "" {
			p.apiKey.Store(ptrTo(val))
		}
	}

	p.log.Debug("PHP-SDK version: " + phpSdkVersion)
	worker.SetStickyWorkflowCacheSize(p.config.CacheSize)

	dialOpts := make([]grpc.DialOption, 0, 2)
	dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(rewriteNameAndVersion(phpSdkVersion)))
	if os.Getenv("NO_PROXY") != "" {
		dialOpts = append(dialOpts, grpc.WithNoProxy())
	}

	opts := tclient.Options{
		HostPort:       p.config.Address,
		MetricsHandler: p.temporal.mh,
		Namespace:      p.config.Namespace,
		Logger:         logger.NewZapAdapter(p.log),
		DataConverter:  dc,
		ConnectionOptions: tclient.ConnectionOptions{
			TLS:         p.temporal.tlsCfg,
			DialOptions: dialOpts,
			// explicitly disable TLS if no config provided
			// because we're always using NewApiKeyDynamicCredentials which leads (from sdk-go 1.39) to TLS by default
			TLSDisabled: p.config.TLS == nil,
		},
		Credentials: tclient.NewAPIKeyDynamicCredentials(func(context.Context) (string, error) {
			if p.apiKey.Load() != nil {
				return *p.apiKey.Load(), nil
			}

			return "", nil
		}),
	}

	var err error
	p.temporal.client, err = tclient.Dial(opts)
	if err != nil {
		return err
	}

	p.log.Info("connected to temporal server", zap.String("address", p.config.Address))

	return nil
}

func rewriteNameAndVersion(phpSdkVersion string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if md == nil || !ok {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		md.Set(clientNameHeaderName, clientNameHeaderValue)
		md.Set(clientVersionHeaderName, phpSdkVersion)

		ctx = metadata.NewOutgoingContext(ctx, md)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
