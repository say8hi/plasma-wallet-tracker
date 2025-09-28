package usecase

import (
	"github.com/say8hi/plasma-wallet-tracker/internal/domain"
	"go.uber.org/zap"
)

type CommandHandler struct {
	walletTracker *WalletTracker
	logger        *zap.Logger
}

func NewCommandHandler(walletTracker *WalletTracker, logger *zap.Logger) *CommandHandler {
	return &CommandHandler{
		walletTracker: walletTracker,
		logger:        logger,
	}
}

func (ch *CommandHandler) HandleCommand(cmd domain.Command) {
	ch.logger.Info("Received command",
		zap.String("type", string(cmd.Type)),
		zap.String("wallet", string(cmd.WalletAddress)),
		zap.Int64("user_id", int64(cmd.UserID)),
	)

	var err error
	switch cmd.Type {
	case domain.AddWalletCommand:
		err = ch.walletTracker.AddWallet(cmd.WalletAddress, cmd.UserID)
	case domain.RemoveWalletCommand:
		err = ch.walletTracker.RemoveWallet(cmd.WalletAddress, cmd.UserID)
	default:
		ch.logger.Error("Unknown command type", zap.String("type", string(cmd.Type)))
		return
	}

	if err != nil {
		ch.logger.Error("Failed to handle command",
			zap.String("type", string(cmd.Type)),
			zap.Error(err),
		)
	}
}
