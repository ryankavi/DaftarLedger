package models

import "time"

type User struct {
	UserID    string
	Email     string
	CreatedAt time.Time
}

type Account struct {
	AccountID   string
	AccountType string
	OwnerID     string
	Currency    string
	CreatedAt   time.Time
}

type Entry struct {
	EntryID       string
	TransactionID string
	AccountID     string
	Amount        int64
	Currency      string
	Direction     string
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
	TransactionStatus      string
	PostedAt               time.Time
	CreatedAt              time.Time
}
