package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ERC20 Transfer event signature
var transferEventSignature = common.HexToHash(
	"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
)

// ERC20 ABI for basic operations
const ERC20ABI = `[
	{
		"constant": true,
		"inputs": [],
		"name": "symbol",
		"outputs": [{"name": "", "type": "string"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals", 
		"outputs": [{"name": "", "type": "uint8"}],
		"type": "function"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"}, 
			{"indexed": false, "name": "value", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}
]`

type ERC20Helper struct {
	client *PlasmaClient
	abi    abi.ABI
}

func NewERC20Helper(client *PlasmaClient) (*ERC20Helper, error) {
	contractABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, err
	}

	return &ERC20Helper{
		client: client,
		abi:    contractABI,
	}, nil
}

func (e *ERC20Helper) GetTokenSymbol(
	ctx context.Context,
	tokenAddress common.Address,
) (string, error) {
	data, err := e.abi.Pack("symbol")
	if err != nil {
		return "", err
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	result, err := e.client.rpcClient.CallContract(ctx, msg, nil)
	if err != nil {
		return "", err
	}

	var symbol string
	err = e.abi.UnpackIntoInterface(&symbol, "symbol", result)
	if err != nil {
		return "", err
	}

	return symbol, nil
}

func (e *ERC20Helper) GetTokenDecimals(
	ctx context.Context,
	tokenAddress common.Address,
) (uint8, error) {
	data, err := e.abi.Pack("decimals")
	if err != nil {
		return 0, err
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	result, err := e.client.rpcClient.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, err
	}

	var decimals uint8
	err = e.abi.UnpackIntoInterface(&decimals, "decimals", result)
	if err != nil {
		return 0, err
	}

	return decimals, nil
}

func (e *ERC20Helper) ParseTransferEvent(
	log *types.Log,
) (from, to common.Address, value *big.Int, err error) {
	if len(log.Topics) < 3 {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid transfer event")
	}

	from = common.HexToAddress(log.Topics[1].Hex())
	to = common.HexToAddress(log.Topics[2].Hex())
	value = new(big.Int).SetBytes(log.Data)

	return from, to, value, nil
}

func IsERC20Transfer(log *types.Log) bool {
	return len(log.Topics) > 0 && log.Topics[0] == transferEventSignature
}
