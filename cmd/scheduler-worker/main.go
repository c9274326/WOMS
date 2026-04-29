package main

import (
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	brokers := env("KAFKA_BROKERS", "kafka:9092")
	topic := env("KAFKA_SCHEDULE_TOPIC", "woms.schedule.jobs")
	group := env("KAFKA_CONSUMER_GROUP", "woms-scheduler-workers")

	log.Printf("scheduler worker starting brokers=%s topic=%s group=%s", brokers, topic, group)
	log.Print("foundation worker is deployment-ready; Kafka consumer wiring is the next implementation slice")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		log.Printf("scheduler worker heartbeat brokers=%s topic=%s", brokers, topic)
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
