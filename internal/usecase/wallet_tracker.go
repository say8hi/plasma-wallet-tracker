package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/say8hi/plasma-wallet-tracker/internal/domain"

	"go.uber.org/zap"
)

type WalletTracker struct {
	blockchainClient domain.BlockchainClient
	publisher        domain.Publisher
	logger           *zap.Logger

	// Active listeners map: wallet address -> listener context
	listeners map[domain.WalletAddress]context.CancelFunc
	// Subscribers map: wallet address -> list of user IDs
	subscribers map[domain.WalletAddress][]domain.UserID
	mu          sync.RWMutex
}

func NewWalletTracker(
	blockchainClient domain.BlockchainClient,
	publisher domain.Publisher,
	logger *zap.Logger,
) *WalletTracker {
	return &WalletTracker{
		blockchainClient: blockchainClient,
		publisher:        publisher,
		logger:           logger,
		listeners:        make(map[domain.WalletAddress]context.CancelFunc),
		subscribers:      make(map[domain.WalletAddress][]domain.UserID),
	}
}

func (wt *WalletTracker) Start(ctx context.Context) {
	wt.logger.Info("Starting wallet tracker service")
	<-ctx.Done()
	wt.logger.Info("Stopping wallet tracker service")
	wt.stopAllListeners()
}

func (wt *WalletTracker) AddWallet(walletAddress domain.WalletAddress, userID domain.UserID) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	// Add user to subscribers list
	wt.subscribers[walletAddress] = append(wt.subscribers[walletAddress], userID)

	// Start listener if it doesn't exist
	if _, exists := wt.listeners[walletAddress]; !exists {
		ctx, cancel := context.WithCancel(context.Background())
		wt.listeners[walletAddress] = cancel

		go wt.startWalletListener(ctx, walletAddress)

		wt.logger.Info("Started listener for wallet",
			zap.String("wallet", string(walletAddress)),
			zap.Int64("user_id", int64(userID)),
		)
	}

	return nil
}

func (wt *WalletTracker) RemoveWallet(
	walletAddress domain.WalletAddress,
	userID domain.UserID,
) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	// Remove user from subscribers list
	subscribers := wt.subscribers[walletAddress]
	for i, id := range subscribers {
		if id == userID {
			wt.subscribers[walletAddress] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}

	// Stop listener if no subscribers left
	if len(wt.subscribers[walletAddress]) == 0 {
		if cancel, exists := wt.listeners[walletAddress]; exists {
			cancel()
			delete(wt.listeners, walletAddress)
			delete(wt.subscribers, walletAddress)

			wt.logger.Info("Stopped listener for wallet",
				zap.String("wallet", string(walletAddress)),
			)
		}
	}

	return nil
}

func (wt *WalletTracker) startWalletListener(
	ctx context.Context,
	walletAddress domain.WalletAddress,
) {
	wt.logger.Info("Starting wallet listener", zap.String("wallet", string(walletAddress)))

	txChan, err := wt.blockchainClient.SubscribeToAddress(ctx, walletAddress)
	if err != nil {
		wt.logger.Error("Failed to subscribe to wallet",
			zap.String("wallet", string(walletAddress)),
			zap.Error(err),
		)
		return
	}

	for {
		select {
		case <-ctx.Done():
			wt.logger.Info("Wallet listener stopped", zap.String("wallet", string(walletAddress)))
			return
		case tx := <-txChan:
			wt.handleTransaction(ctx, walletAddress, tx)
		}
	}
}

func (wt *WalletTracker) handleTransaction(
	ctx context.Context,
	walletAddress domain.WalletAddress,
	tx domain.Transaction,
) {
	wt.mu.RLock()
	subscribers := make([]domain.UserID, len(wt.subscribers[walletAddress]))
	copy(subscribers, wt.subscribers[walletAddress])
	wt.mu.RUnlock()

	if len(subscribers) == 0 {
		return
	}

	notification := domain.WalletNotification{
		WalletAddress: walletAddress,
		Transaction:   tx,
		Subscribers:   subscribers,
		Timestamp:     time.Now(),
	}

	if err := wt.publisher.PublishNotification(ctx, notification); err != nil {
		wt.logger.Error("Failed to publish notification",
			zap.String("wallet", string(walletAddress)),
			zap.String("tx_hash", string(tx.Hash)),
			zap.Error(err),
		)
	} else {
		wt.logger.Info("Published transaction notification",
			zap.String("wallet", string(walletAddress)),
			zap.String("tx_hash", string(tx.Hash)),
			zap.Int("subscribers", len(subscribers)),
		)
	}
}

func (wt *WalletTracker) stopAllListeners() {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	for walletAddress, cancel := range wt.listeners {
		cancel()
		wt.logger.Info("Stopped listener", zap.String("wallet", string(walletAddress)))
	}

	wt.listeners = make(map[domain.WalletAddress]context.CancelFunc)
	wt.subscribers = make(map[domain.WalletAddress][]domain.UserID)
}
