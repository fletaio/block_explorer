package main

import (
	"errors"
)

// account_tx errors
var (
	ErrInvalidSequence             = errors.New("invalid sequence")
	ErrInvalidTransactionSignature = errors.New("invalid transaction signature")
	ErrInvalidSignerCount          = errors.New("invalid signer count")
	ErrInvalidAccountSigner        = errors.New("invalid account signer")
	ErrInvalidLevel                = errors.New("invalid level")
	ErrInvalidAreaType             = errors.New("invalid area type")
	ErrInvalidDemolition           = errors.New("invalid demolition")
	ErrInvalidPosition             = errors.New("invalid position")
	ErrNotAllowed                  = errors.New("not allowed")
	ErrNotExistTile                = errors.New("not exist tile")
	ErrExistAddress                = errors.New("exist address")
	ErrInsufficientResource        = errors.New("insufficient resource")
)
