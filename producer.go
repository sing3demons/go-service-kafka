package main

import (
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/sing3demons/service-go/model"
)

func produceSalesRecordToKafka(records []model.SalesRecord, kafkaBrokers []string, kafkaTopic string) error {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(kafkaBrokers, config)
	if err != nil {
		return fmt.Errorf("error creating Kafka producer: %v", err)
	}
	defer producer.Close()

	for index, record := range records {
		message, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("error encoding record %d as JSON: %v", index, err)
		}

		// Create a Kafka message with the JSON-encoded data
		kafkaMessage := &sarama.ProducerMessage{
			Topic: kafkaTopic,
			Value: sarama.StringEncoder(message),
		}

		// Send the message to Kafka
		_, _, err = producer.SendMessage(kafkaMessage)
		if err != nil {
			return fmt.Errorf("error producing message for record %d: %v", index, err)
		}
	}

	return nil
}
