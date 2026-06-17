package models

import "time"

type EntryDirection string

const (
	DirectionDebit  EntryDirection = "DEBIT"
	DirectionCredit EntryDirection = "CREDIT"
)

type AccountType string

const (
	AccountUserCash       AccountType = "USER_CASH"
	AccountExternal       AccountType = "EXTERNAL"
	AccountCardSettlement AccountType = "CARD_SETTLEMENT"
	AccountACHClearing    AccountType = "ACH_CLEARING"
	AccountFeeRevenue     AccountType = "FEE_REVENUE"
	AccountTreasury       AccountType = "TREASURY"
)

type AccountStatus string

const (
	AccountStatusOpen   AccountStatus = "OPEN"
	AccountStatusFrozen AccountStatus = "FROZEN"
	AccountStatusClosed AccountStatus = "CLOSED"
)

type TransactionStatus string

const (
	StatusPending  TransactionStatus = "PENDING"
	StatusPosted   TransactionStatus = "POSTED"
	StatusReversed TransactionStatus = "REVERSED"
)

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

type User struct {
	UserID       string
	Email        string
	PasswordHash string
	Role         UserRole
	CreatedAt    time.Time
}

type Account struct {
	AccountID     string
	AccountType   AccountType
	AccountStatus AccountStatus
	OwnerID       string
	Currency      string
	CreatedAt     time.Time
}

type Entry struct {
	EntryID       string
	TransactionID string
	AccountID     string
	Amount        int64
	Currency      string
	Direction     EntryDirection
	Memo          *string
	CreatedAt     time.Time
	EffectiveAt   time.Time
}

type Transaction struct {
	TransactionID          string
	ExternalID             *string
	IdempotencyKey         string
	TransactionType        string
	TransactionDescription *string
	TransactionStatus      TransactionStatus
	PostedAt               *time.Time
	CreatedAt              time.Time
}
