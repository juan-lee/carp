package bus

import (
	"context"
	"errors"
	"fmt"

	servicebus "github.com/Azure/azure-service-bus-go"
)

type ServiceBusListener struct {
	Config *ListenerConfig
}

type ListenerConfig struct {
	Region                     string
	Environment                string
	UnderlayID                 string // ex: region/<region>/underlay/<id>
	ServiceBusNamespace        string
	ServiceBusConnectionString string
}

func NewListener(cfg *ListenerConfig) Listener {
	return &ServiceBusListener{cfg}
}

// Listen waits for a message from the Service Bus Topic subscription
func (l *ServiceBusListener) Listen(ctx context.Context, handle Handle) error {
	// Setup necessary SB resources
	namespace, err := getNamespace(l.Config.ServiceBusConnectionString)
	if err != nil {
		return fmt.Errorf("failed to get namespace from provided ServiceBus connection string: %w", err)
	}
	topicEntity, err := getTopicEntity(ctx, l.Config.Environment, l.Config.Region, namespace)
	if err != nil {
		return fmt.Errorf("failed to get topicEntity: %w", err)
	}
	subscriptionEntity, err := getSubscriptionEntity(ctx, l.Config.UnderlayID, namespace, topicEntity)
	if err != nil {
		return fmt.Errorf("failed to get subscriptionEntity: %w", err)
	}

	// Generate new topic client
	topic, err := namespace.NewTopic(topicEntity.Name)
	if err != nil {
		return fmt.Errorf("failed to create new topic %s: %w", topicEntity.Name, err)
	}
	defer func() {
		_ = topic.Close(ctx)
	}()

	// Generate new subscription client
	sub, err := topic.NewSubscription(subscriptionEntity.Name)
	if err != nil {
		return fmt.Errorf("failed to create new subscription %s: %w", subscriptionEntity.Name, err)
	}
	subReceiver, err := sub.NewReceiver(ctx)
	if err != nil {
		return fmt.Errorf("failed to create new subscription receiver %s: %w", subReceiver.Name, err)
	}
	// Create a handle class that has that function
	listenerHandle := subReceiver.Listen(ctx, servicebus.HandlerFunc(
		func(ctx context.Context, message *servicebus.Message) error {
			err := handle(ctx, string(message.Data))
			if err != nil {
				err = message.Abandon(ctx)
				return err
			}
			return message.Complete(ctx)
		},
	))
	<-listenerHandle.Done()

	if err := subReceiver.Close(ctx); err != nil {
		return fmt.Errorf("error shutting down service bus subscription. %w", err)
	}
	return listenerHandle.Err()
}

func getNamespace(connStr string) (*servicebus.Namespace, error) {
	if connStr == "" {
		return nil, errors.New("no Service Bus connection string provided")
	}
	// Create a client to communicate with a Service Bus Namespace.
	namespace, err := servicebus.NewNamespace(servicebus.NamespaceWithConnectionString(connStr))
	if err != nil {
		return nil, err
	}
	return namespace, nil
}

func getTopicEntity(ctx context.Context, environment, region string, namespace *servicebus.Namespace) (*servicebus.TopicEntity, error) {
	topicManager := namespace.NewTopicManager()
	topicEntity, err := ensureTopic(ctx, topicManager, fmt.Sprintf("%s-%s", environment, region))
	if err != nil {
		return nil, err
	}

	return topicEntity, nil
}

func getSubscriptionEntity(
	ctx context.Context,
	underlayID string,
	ns *servicebus.Namespace,
	te *servicebus.TopicEntity) (*servicebus.SubscriptionEntity, error) {
	subscriptionManager, err := ns.NewSubscriptionManager(te.Name)
	if err != nil {
		return nil, err
	}

	subEntity, err := ensureSubscription(ctx, subscriptionManager, underlayID)
	if err != nil {
		return nil, err
	}

	return subEntity, nil
}

func ensureTopic(ctx context.Context, tm *servicebus.TopicManager, name string) (*servicebus.TopicEntity, error) {
	te, err := tm.Get(ctx, name)
	if err == nil {
		return te, nil
	}

	return tm.Put(ctx, name)
}

func ensureSubscription(ctx context.Context, sm *servicebus.SubscriptionManager, name string) (*servicebus.SubscriptionEntity, error) {
	subEntity, err := sm.Get(ctx, name)
	if err == nil {
		return subEntity, nil
	}

	subEntity, err = sm.Put(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create subscription %s", name)
	}

	err = ensureFilterRule(ctx, sm, name)
	if err != nil {
		return nil, err
	}
	return subEntity, err
}

func ensureFilterRule(ctx context.Context, sm *servicebus.SubscriptionManager, name string) error {
	// This may result in a concurrency issue where the receiver may get a message before the filter rule is in place
	// probably disable the disable receive while updating?
	// maybe it's ok, we would just get messages that aren't for us, they should fail, and go on the deadletter.

	rules, err := sm.ListRules(ctx, name)
	if err != nil {
		return err
	}
	sqlFilter := fmt.Sprintf("destinationId LIKE '%s'", name)
	ruleName := "destinationIdFilter"

	for _, rule := range rules {
		sm.DeleteRule(ctx, name, rule.Name)
	}

	_, err = sm.PutRule(ctx, name, ruleName, servicebus.SQLFilter{Expression: sqlFilter})
	return err
}
