package redis

import (
	"context"
	"encoding/json"

	"github.com/say8hi/plasma-wallet-tracker/internal/domain"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Subscriber struct {
	client  *redis.Client
	channel string
	logger  *zap.Logger
}

func NewSubscriber(redisClient *Client, logger *zap.Logger) *Subscriber {
	return &Subscriber{
		client:  redisClient.GetRedisClient(),
		channel: "wallet_commands", // TODO: get from config
		logger:  logger,
	}
}

func (s *Subscriber) SubscribeCommands(ctx context.Context, handler func(domain.Command)) error {
	pubsub := s.client.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	s.logger.Info("Subscribed to commands channel", zap.String("channel", s.channel))

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Command subscriber stopped")
			return ctx.Err()
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var cmd domain.Command
			if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
				s.logger.Error("Failed to unmarshal command",
					zap.String("payload", msg.Payload),
					zap.Error(err),
				)
				continue
			}

			s.logger.Debug("Received command",
				zap.String("type", string(cmd.Type)),
				zap.String("wallet", string(cmd.WalletAddress)),
				zap.Int64("user_id", int64(cmd.UserID)),
			)

			// Handle command in separate goroutine to avoid blocking
			go handler(cmd)
		}
	}
}
