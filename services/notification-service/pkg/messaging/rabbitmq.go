package messaging

import (
	"context"
	"fmt"
	"time"

	"notification-service/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.RabbitMQConfig
}

func NewRabbitMQClient(cfg *config.RabbitMQConfig) (*RabbitMQClient, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		cfg.User, cfg.Password, cfg.Host, cfg.Port)

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	return &RabbitMQClient{
		conn:    conn,
		channel: channel,
		config:  cfg,
	}, nil
}

func (c *RabbitMQClient) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *RabbitMQClient) DeclareQueue(name string) (amqp.Queue, error) {
	return c.channel.QueueDeclare(
		name,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func (c *RabbitMQClient) Publish(ctx context.Context, queueName string, body []byte) error {
	_, err := c.DeclareQueue(queueName)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	return c.channel.PublishWithContext(
		ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)
}

func (c *RabbitMQClient) Consume(queueName string) (<-chan amqp.Delivery, error) {
	_, err := c.DeclareQueue(queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return c.channel.Consume(
		queueName,
		"",    // consumer
		false, // auto-ack (changed to false for manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
}

func (c *RabbitMQClient) GetChannel() *amqp.Channel {
	return c.channel
}