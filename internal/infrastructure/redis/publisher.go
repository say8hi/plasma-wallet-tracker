package redis

import (
	"context"
	"encoding/json"

	"github.com/say8hi/plasma-wallet-tracker/internal/domain"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Publisher struct {
	client  *redis.Client
	channel string
	logger  *zap.Logger
}

func NewPublisher(redisClient *Client, logger *zap.Logger) *Publisher {
	return &Publisher{
		client:  redisClient.GetRedisClient(),
		channel: "wallet_notifications", // TODO: get from config
		logger:  logger,
	}
}

func (p *Publisher) PublishNotification(
	ctx context.Context,
	notification domain.WalletNotification,
) error {
	data, err := json.Marshal(notification)
	if err != nil {
		p.logger.Error("Failed to marshal notification", zap.Error(err))
		return err
	}

	err = p.client.Publish(ctx, p.channel, data).Err()
	if err != nil {
		p.logger.Error("Failed to publish notification to Redis",
			zap.String("channel", p.channel),
			zap.Error(err),
		)
		return err
	}

	p.logger.Debug("Published notification",
		zap.String("channel", p.channel),
		zap.String("wallet", string(notification.WalletAddress)),
		zap.Int("subscribers", len(notification.Subscribers)),
	)

	return nil
}
