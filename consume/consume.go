package consume

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/sing3demons/service-go/model"
	"go.mongodb.org/mongo-driver/mongo"
)

func ConsumeAndInsertToMongoDB(kafkaBrokers []string, kafkaTopic string, groupID string, db *mongo.Database, collectionName string) {
	fmt.Println("ConsumeAndInsertToMongoDB")
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumer, err := sarama.NewConsumerGroup(kafkaBrokers, groupID, config)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	collection := db.Collection(collectionName)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			if err := consumer.Consume(ctx, []string{kafkaTopic}, &salesRecordConsumer{collection: collection}); err != nil {
				log.Fatal(err)
			}
			// Check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Handle graceful shutdown
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigterm:
		log.Println("Received termination signal. Initiating shutdown...")
		cancel()
	case <-ctx.Done():
	}

	// Wait for the consumer to finish processing
	wg.Wait()
}

type salesRecordConsumer struct {
	collection *mongo.Collection
}

func (c *salesRecordConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *salesRecordConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (c *salesRecordConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var salesRecord model.SalesRecord
		err := json.Unmarshal(message.Value, &salesRecord)
		if err != nil {
			log.Printf("Error decoding message: %v", err)
			continue
		}

		// Insert the sales record into MongoDB
		_, err = c.collection.InsertOne(context.TODO(), salesRecord)
		if err != nil {
			log.Printf("Error inserting into MongoDB: %v", err)
		}

		// Mark the message as processed
		session.MarkMessage(message, "")
	}

	return nil
}
