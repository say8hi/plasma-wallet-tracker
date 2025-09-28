package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/say8hi/plasma-wallet-tracker/config"
	"github.com/say8hi/plasma-wallet-tracker/internal/domain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

type PlasmaClient struct {
	rpcClient  *ethclient.Client
	wsClient   *ethclient.Client
	chainID    *big.Int
	logger     *zap.Logger
	tokenCache map[common.Address]string
	mu         sync.RWMutex
}

func NewPlasmaClient(cfg config.BlockchainConfig) (*PlasmaClient, error) {
	// Initialize RPC client
	rpcClient, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	// Initialize WebSocket client
	wsClient, err := ethclient.Dial(cfg.WSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()

	return &PlasmaClient{
		rpcClient:  rpcClient,
		wsClient:   wsClient,
		chainID:    big.NewInt(cfg.ChainID),
		logger:     logger,
		tokenCache: make(map[common.Address]string),
	}, nil
}

func (pc *PlasmaClient) SubscribeToAddress(
	ctx context.Context,
	address domain.WalletAddress,
) (<-chan domain.Transaction, error) {
	txChan := make(chan domain.Transaction, 100)
	walletAddr := common.HexToAddress(string(address))

	// Subscribe to new heads to get new blocks
	headers := make(chan *types.Header)
	sub, err := pc.wsClient.SubscribeNewHead(ctx, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to new heads: %w", err)
	}

	go func() {
		defer close(txChan)
		defer sub.Unsubscribe()

		pc.logger.Info("Started monitoring wallet",
			zap.String("address", string(address)))

		for {
			select {
			case <-ctx.Done():
				pc.logger.Info("Stopped monitoring wallet",
					zap.String("address", string(address)))
				return
			case err := <-sub.Err():
				pc.logger.Error("Subscription error",
					zap.String("address", string(address)),
					zap.Error(err))
				return
			case header := <-headers:
				// Check transactions in the new block
				pc.processBlockForAddress(ctx, header, walletAddr, txChan)
			}
		}
	}()

	return txChan, nil
}

func (pc *PlasmaClient) processBlockForAddress(
	ctx context.Context,
	header *types.Header,
	address common.Address,
	txChan chan<- domain.Transaction,
) {
	block, err := pc.rpcClient.BlockByHash(ctx, header.Hash())
	if err != nil {
		pc.logger.Error("Failed to get block",
			zap.String("hash", header.Hash().Hex()),
			zap.Error(err))
		return
	}

	// Check each transaction in the block
	for _, tx := range block.Transactions() {
		// Get receipt to analyze logs
		receipt, err := pc.rpcClient.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			continue // Skip if we can't get receipt
		}

		// Check if our address is involved in the transaction
		if pc.isAddressInvolved(tx, receipt, address) {
			// Extract all transfers and create domain transaction
			domainTx := pc.createDomainTransaction(tx, receipt, block.Time())

			// Filter transfers for the watched address
			relevantTransfers := pc.filterTransfersForAddress(domainTx.Transfers, address)

			if len(relevantTransfers) > 0 {
				domainTx.Transfers = relevantTransfers

				select {
				case txChan <- domainTx:
					pc.logger.Info("Detected transaction with transfers",
						zap.String("tx_hash", tx.Hash().Hex()),
						zap.Int("transfers", len(relevantTransfers)),
						zap.String("address", address.Hex()))
				case <-ctx.Done():
					return
				default:
					pc.logger.Warn("Channel full, dropping transaction",
						zap.String("hash", tx.Hash().Hex()))
				}
			}
		}
	}
}

func (pc *PlasmaClient) isAddressInvolved(
	tx *types.Transaction,
	receipt *types.Receipt,
	address common.Address,
) bool {
	// 1. Check direct involvement (from/to)
	if from, err := types.Sender(types.NewEIP155Signer(pc.chainID), tx); err == nil &&
		from == address {
		return true
	}
	if tx.To() != nil && *tx.To() == address {
		return true
	}

	// 2. Check involvement in Transfer events
	for _, log := range receipt.Logs {
		if len(log.Topics) >= 3 && log.Topics[0] == transferEventSignature {
			from := common.HexToAddress(log.Topics[1].Hex())
			to := common.HexToAddress(log.Topics[2].Hex())

			if from == address || to == address {
				return true
			}
		}
	}

	return false
}

func (pc *PlasmaClient) createDomainTransaction(
	tx *types.Transaction,
	receipt *types.Receipt,
	blockTime uint64,
) domain.Transaction {
	// Get sender address
	fromAddr, _ := types.Sender(types.NewEIP155Signer(pc.chainID), tx)

	// Get recipient address
	toAddr := ""
	if tx.To() != nil {
		toAddr = tx.To().Hex()
	}

	// Extract all transfers
	transfers := pc.extractAllTransfers(tx, receipt)

	return domain.Transaction{
		Hash:        domain.TransactionHash(tx.Hash().Hex()),
		From:        domain.WalletAddress(fromAddr.Hex()),
		To:          domain.WalletAddress(toAddr),
		BlockNumber: receipt.BlockNumber.Uint64(),
		Timestamp:   time.Unix(int64(blockTime), 0),
		GasUsed:     receipt.GasUsed,
		GasPrice:    tx.GasPrice(),
		Transfers:   transfers,
	}
}

func (pc *PlasmaClient) extractAllTransfers(
	tx *types.Transaction,
	receipt *types.Receipt,
) []domain.Transfer {
	var transfers []domain.Transfer

	// 1. Native transfer (if value > 0)
	if tx.Value().Cmp(big.NewInt(0)) > 0 {
		fromAddr, _ := types.Sender(types.NewEIP155Signer(pc.chainID), tx)
		toAddr := ""
		if tx.To() != nil {
			toAddr = tx.To().Hex()
		}

		transfer := domain.Transfer{
			TxHash:       domain.TransactionHash(tx.Hash().Hex()),
			From:         domain.WalletAddress(fromAddr.Hex()),
			To:           domain.WalletAddress(toAddr),
			Value:        tx.Value(),
			TokenSymbol:  "XPL",
			TokenAddress: "0x0000000000000000000000000000000000000000",
			LogIndex:     -1, // Native transfer doesn't have log index
		}
		transfers = append(transfers, transfer)
	}

	// 2. ERC-20 transfers from logs
	for i, log := range receipt.Logs {
		if len(log.Topics) >= 3 && log.Topics[0] == transferEventSignature {
			from := common.HexToAddress(log.Topics[1].Hex())
			to := common.HexToAddress(log.Topics[2].Hex())
			value := new(big.Int).SetBytes(log.Data)

			tokenSymbol := pc.getTokenSymbol(context.Background(), log.Address)

			transfer := domain.Transfer{
				TxHash:       domain.TransactionHash(tx.Hash().Hex()),
				From:         domain.WalletAddress(from.Hex()),
				To:           domain.WalletAddress(to.Hex()),
				Value:        value,
				TokenSymbol:  tokenSymbol,
				TokenAddress: log.Address.Hex(),
				LogIndex:     i,
			}
			transfers = append(transfers, transfer)
		}
	}

	return transfers
}

func (pc *PlasmaClient) filterTransfersForAddress(
	transfers []domain.Transfer,
	address common.Address,
) []domain.Transfer {
	var relevantTransfers []domain.Transfer

	for _, transfer := range transfers {
		fromAddr := common.HexToAddress(string(transfer.From))
		toAddr := common.HexToAddress(string(transfer.To))

		if fromAddr == address || toAddr == address {
			relevantTransfers = append(relevantTransfers, transfer)
		}
	}

	return relevantTransfers
}

func (pc *PlasmaClient) getTokenSymbol(ctx context.Context, tokenAddress common.Address) string {
	pc.mu.RLock()
	if symbol, exists := pc.tokenCache[tokenAddress]; exists {
		pc.mu.RUnlock()
		return symbol
	}
	pc.mu.RUnlock()

	// Special cases for known tokens
	switch tokenAddress.Hex() {
	case "0x0000000000000000000000000000000000000000":
		return "XPL"
	case "0xa0b86a33e6ba0c74d75c9abfd35e5e0b1bcceb83": // Example WXPL
		return "WXPL"
	}

	// Try to get symbol via ERC-20
	helper, err := NewERC20Helper(pc)
	if err != nil {
		return tokenAddress.Hex()[:8]
	}

	symbol, err := helper.GetTokenSymbol(ctx, tokenAddress)
	if err != nil {
		symbol = tokenAddress.Hex()[:8]
	}

	// Cache the result
	pc.mu.Lock()
	pc.tokenCache[tokenAddress] = symbol
	pc.mu.Unlock()

	return symbol
}

func (pc *PlasmaClient) GetLatestBlock(ctx context.Context) (uint64, error) {
	block, err := pc.rpcClient.BlockByNumber(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}
	return block.NumberU64(), nil
}

func (pc *PlasmaClient) GetTransaction(
	ctx context.Context,
	hash domain.TransactionHash,
) (*domain.Transaction, error) {
	txHash := common.HexToHash(string(hash))

	tx, isPending, err := pc.rpcClient.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	if isPending {
		return nil, fmt.Errorf("transaction is pending")
	}

	receipt, err := pc.rpcClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	block, err := pc.rpcClient.BlockByHash(ctx, receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	domainTx := pc.createDomainTransaction(tx, receipt, block.Time())
	return &domainTx, nil
}

func (pc *PlasmaClient) GetTransfersForAddress(
	ctx context.Context,
	txHash domain.TransactionHash,
	address domain.WalletAddress,
) ([]domain.Transfer, error) {
	tx, err := pc.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, err
	}

	watchedAddr := common.HexToAddress(string(address))
	return pc.filterTransfersForAddress(tx.Transfers, watchedAddr), nil
}

func (pc *PlasmaClient) HealthCheck(ctx context.Context) error {
	_, err := pc.GetLatestBlock(ctx)
	return err
}

func (pc *PlasmaClient) Close() {
	if pc.rpcClient != nil {
		pc.rpcClient.Close()
	}
	if pc.wsClient != nil {
		pc.wsClient.Close()
	}
}
