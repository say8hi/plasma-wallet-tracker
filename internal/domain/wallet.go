package domain

import (
	"context"
	"math/big"
	"time"
)

type WalletAddress string

type UserID int64

type TransactionHash string

// WalletSubscription represents a user's subscription to a wallet
type WalletSubscription struct {
	WalletAddress WalletAddress `json:"wallet_address"`
	UserID        UserID        `json:"user_id"`
	CreatedAt     time.Time     `json:"created_at"`
}

// Transfer represents a single token transfer within a transaction
type Transfer struct {
	TxHash       TransactionHash `json:"tx_hash"`
	From         WalletAddress   `json:"from"`
	To           WalletAddress   `json:"to"`
	Value        *big.Int        `json:"value"`
	TokenSymbol  string          `json:"token_symbol"`
	TokenAddress string          `json:"token_address"`
	LogIndex     int             `json:"log_index"`
}

// Transaction represents a blockchain transaction with multiple transfers
type Transaction struct {
	Hash        TransactionHash `json:"hash"`
	From        WalletAddress   `json:"from"` // Transaction sender
	To          WalletAddress   `json:"to"`   // Transaction recipient (contract)
	BlockNumber uint64          `json:"block_number"`
	Timestamp   time.Time       `json:"timestamp"`
	GasUsed     uint64          `json:"gas_used"`
	GasPrice    *big.Int        `json:"gas_price"`
	Transfers   []Transfer      `json:"transfers"` // All transfers in this tx
}

// WalletNotification represents a notification to be sent
type WalletNotification struct {
	WalletAddress WalletAddress `json:"wallet_address"`
	Transaction   Transaction   `json:"transaction"`
	Transfers     []Transfer    `json:"transfers"` // Only transfers involving watched address
	Subscribers   []UserID      `json:"subscribers"`
	Timestamp     time.Time     `json:"timestamp"`
}

// Command represents a wallet management command
type Command struct {
	Type          CommandType   `json:"type"`
	WalletAddress WalletAddress `json:"wallet_address"`
	UserID        UserID        `json:"user_id"`
	Timestamp     time.Time     `json:"timestamp"`
}

type CommandType string

const (
	AddWalletCommand    CommandType = "add_wallet"
	RemoveWalletCommand CommandType = "remove_wallet"
)

// BlockchainClient interface for blockchain operations
type BlockchainClient interface {
	// SubscribeToAddress monitors address and returns channel of transactions
	// containing transfers that involve the specified address
	SubscribeToAddress(ctx context.Context, address WalletAddress) (<-chan Transaction, error)

	// GetLatestBlock returns the latest block number
	GetLatestBlock(ctx context.Context) (uint64, error)

	// GetTransaction returns transaction details with all transfers
	GetTransaction(ctx context.Context, hash TransactionHash) (*Transaction, error)

	// GetTransfersForAddress returns all transfers involving address in a transaction
	GetTransfersForAddress(
		ctx context.Context,
		txHash TransactionHash,
		address WalletAddress,
	) ([]Transfer, error)
}

// Publisher interface for publishing notifications
type Publisher interface {
	PublishNotification(ctx context.Context, notification WalletNotification) error
}

// Subscriber interface for receiving commands
type Subscriber interface {
	SubscribeCommands(ctx context.Context, handler func(Command)) error
}

// WalletRepository interface for wallet data persistence
type WalletRepository interface {
	AddSubscription(ctx context.Context, subscription WalletSubscription) error
	RemoveSubscription(ctx context.Context, walletAddress WalletAddress, userID UserID) error
	GetSubscribers(ctx context.Context, walletAddress WalletAddress) ([]UserID, error)
	GetAllWallets(ctx context.Context) ([]WalletAddress, error)
}
