package kafka

import (
	"context"
	//	"flag"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"time"
	//	"strings"
	"github.com/Shopify/sarama"
	"sync"
	"syscall"
)

// Sarama configuration options
var (
	//	brokers  = ""
	version = "2.1.1"
	//	group    = ""
	//	topics   = ""
	assignor       = "roundrobin"
	oldest         = true
	verbose        = false
	Client         sarama.ConsumerGroup
	WG             *sync.WaitGroup
	StopClient     = make(chan bool)
	MessageHandler HandlerFunc
)

type HandlerFunc func(string, time.Time, []byte)

func init() {
	//	flag.StringVar(&brokers, "brokers", "", "Kafka bootstrap brokers to connect to, as a comma separated list")
	//	flag.StringVar(&group, "group", "", "Kafka consumer group definition")
	//	flag.StringVar(&version, "version", "2.1.1", "Kafka cluster version")
	//	flag.StringVar(&topics, "topics", "", "Kafka topics to be consumed, as a comma separated list")
	//	flag.StringVar(&assignor, "assignor", "range", "Consumer group partition assignment strategy (range, roundrobin, sticky)")
	//	flag.BoolVar(&oldest, "oldest", true, "Kafka consumer consume initial offset from oldest")
	//	flag.BoolVar(&verbose, "verbose", false, "Sarama logging")
	//	flag.Parse()
	/*
		if len(brokers) == 0 {
			panic("no Kafka bootstrap brokers defined, please set the -brokers flag")
		}

		if len(topics) == 0 {
			panic("no topics given to be consumed, please set the -topics flag")
		}

		if len(group) == 0 {
			panic("no Kafka consumer group defined, please set the -group flag")
		}
	*/
}

func StartConsumer(brokers []string, topics []string, group string, handler func(string, time.Time, []byte)) error {
	log.Info("Starting a new Sarama consumer")
	var err error
	MessageHandler = handler
	/*
		if verbose {
			sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
		}
	*/
	var kafkaversion sarama.KafkaVersion
	kafkaversion, err = sarama.ParseKafkaVersion(version)
	if err != nil {
		log.Panicf("Error parsing Kafka version: %v", err)
	}

	/**
	 * Construct a new Sarama configuration.
	 * The Kafka cluster version has to be defined before the consumer/producer is initialized.
	 */
	config := sarama.NewConfig()
	config.Version = kafkaversion

	switch assignor {
	case "sticky":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	case "roundrobin":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	case "range":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	default:
		log.Panicf("Unrecognized consumer group partition assignor: %s", assignor)
	}

	if oldest {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	/**
	 * Setup a new Sarama consumer group
	 */
	consumer := Consumer{
		ready: make(chan bool),
	}

	ctx, cancel := context.WithCancel(context.Background())
	Client, err = sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		log.Panicf("Error creating consumer group client: %v", err)
	}

	WG = &sync.WaitGroup{}
	WG.Add(1)

	go func() {
		defer WG.Done()
		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims
			if err = Client.Consume(ctx, topics, &consumer); err != nil {
				log.Panicf("Error from consumer: %v", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			consumer.ready = make(chan bool)
		}
		cancel()
	}()

	<-consumer.ready // Await till the consumer has been set up
	log.Info("Sarama consumer up and running!...")

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		log.Println("terminating: context cancelled")
	//	case <-sigterm:
	//		log.Println("terminating: via signal")
	case <-StopClient:
		return err
	}
	return err
}

func StopConsumer() {
	//	cancel()
	log.Error("Shutting down consumer")
	StopClient <- true
	WG.Wait()
	if err := Client.Close(); err != nil {
		log.Errorf("Error closing client: %v", err)
	}
}

// Consumer represents a Sarama consumer group consumer
type Consumer struct {
	ready chan bool
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		log.Debugf("Message claimed: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic)
		MessageHandler(message.Topic, message.Timestamp, message.Value)
		session.MarkMessage(message, "")
	}

	return nil
}
