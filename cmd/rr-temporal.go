package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/informer"
	"github.com/spiral/roadrunner/v2/plugins/resetter"

	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/rpc"
	"github.com/spiral/roadrunner/v2/plugins/server"
	"github.com/temporalio/roadrunner-temporal/plugins/activity"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/workflow"
)

func main() {
	container, err := endure.NewContainer(nil, endure.RetryOnFail(false))
	if err != nil {
		log.Fatal(err)
	}

	cfg := &config.Viper{}
	cfg.Path = ".rr.yaml"
	cfg.Prefix = "rr"

	err = container.RegisterAll(
		cfg,
		&logger.ZapLogger{},

		// Helpers
		&resetter.Plugin{},
		&informer.Plugin{},

		// PHP application init.
		&server.Plugin{},
		&rpc.Plugin{},

		// Temporal extension.
		&temporal.Plugin{},
		&activity.Plugin{},
		&workflow.Plugin{},
	)

	if err != nil {
		log.Fatal(err)
		return
	}

	err = container.Init()
	if err != nil {
		log.Fatal(err)
	}

	ch, err := container.Serve()
	if err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				err = container.Stop()
				if err != nil {
					log.Fatal(e.Error.Error() + ":" + err.Error())
				}
				log.Fatal(e.Error)
			case <-sig:
				err = container.Stop()
				if err != nil {
					log.Fatal(err)
				}
				return
			case <-stopCh:
				// timeout
				err = container.Stop()
				if err != nil {
					log.Fatal(err)
				}
				return
			}
		}
	}()

	stopCh <- struct{}{}
	wg.Wait()
}
