package domain

import "errors"

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrSubscriptionExists  = errors.New("subscription already exists")
	ErrInvalidAddress      = errors.New("invalid wallet address")
	ErrConnectionFailed    = errors.New("connection failed")
	ErrTransactionNotFound = errors.New("transaction not found")
)
