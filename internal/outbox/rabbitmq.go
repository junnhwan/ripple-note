package outbox

import (
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher publishes events to a RabbitMQ topic exchange.
type RabbitMQPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
	logger   *slog.Logger
	mu       sync.Mutex
}

// NewRabbitMQPublisher connects to RabbitMQ, declares the exchange, and returns a publisher.
func NewRabbitMQPublisher(dsn, exchange string, logger *slog.Logger) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	// Declare a topic exchange so events can be routed by topic name.
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq exchange declare: %w", err)
	}

	logger.Info("rabbitmq connected", "exchange", exchange)
	return &RabbitMQPublisher{
		conn:     conn,
		channel:  ch,
		exchange: exchange,
		logger:   logger,
	}, nil
}

func (p *RabbitMQPublisher) Publish(topic string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.channel.Publish(
		p.exchange, // exchange
		topic,      // routing key (= topic name)
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	)
	if err != nil {
		p.logger.Error("rabbitmq publish failed", "topic", topic, "error", err)
		return fmt.Errorf("rabbitmq publish: %w", err)
	}
	return nil
}

func (p *RabbitMQPublisher) Close() error {
	if err := p.channel.Close(); err != nil {
		return err
	}
	return p.conn.Close()
}
