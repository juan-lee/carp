package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apex/log"
	"github.com/spf13/pflag"

	"github.com/juan-lee/carp/internal/bus"
	"github.com/juan-lee/carp/internal/messages"
	"github.com/juan-lee/carp/internal/messages/workers"
)

var (
	// required arg
	region              = pflag.String("region", "eastus", "region the underlay is in")
	environment         = pflag.String("environment", "prod", "environment the underlay is in (intv2, staging, prod)")
	underlayName        = pflag.String("underlay-name", "hcp-underlay-dev-cx-0", "hcp namespace")
	serviceBusNamespace = pflag.String("service-bus-namespace", "aksglobalgitopssb", "gitops sb namespace")
	deploymentNamespace = pflag.String("deployment-namespace", "gitops-dp", "namespace that these gitops deployment live in")

	// temporarily pass in SAS token until we use /etc/kubernetes/azure.json
	serviceBusConnectionString = pflag.String("service-bus-connection-string", "", "connection string for SB")
)

func main() {
	pflag.Parse()

	sbListenerCfg := &bus.ListenerConfig{
		Region:                     *region,
		Environment:                *environment,
		UnderlayID:                 *underlayName,
		ServiceBusNamespace:        *serviceBusNamespace,
		ServiceBusConnectionString: *serviceBusConnectionString,
	}

	handler := func(ctx context.Context, message string) error {
		log.Info(message)
		return nil
	}
	rec := bus.NewListener(sbListenerCfg)

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	go func() {
		time.Sleep(3 * time.Second)
		sbPublisherConfig := &bus.PublisherConfig{
			Region:                     *region,
			Environment:                *environment,
			UnderlayName:               *underlayName,
			ServiceBusConnectionString: *serviceBusConnectionString,
		}
		publisher, _ := bus.NewPublisher(ctx, sbPublisherConfig)
		command := workers.PutCluster{
			Command: messages.Command{DestinationId: *underlayName},
		}
		publisher.Publish(ctx, command)
		command = workers.PutCluster{
			Command: messages.Command{DestinationId: "bad worker name"},
		}
		publisher.Publish(ctx, command)
	}()

	log.Info("Starting to listen for events...")
	err := rec.Listen(ctx, handler) // sync call
	if err != nil {
		log.Error(err.Error())
	}
}
