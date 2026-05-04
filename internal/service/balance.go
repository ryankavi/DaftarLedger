package service

import "github.com/ryankavi/payclone/internal/models"

// needsFundsCheck reports whether the source account type requires a
// balance check. Rail/platform accounts (EXTERNAL, TREASURY) are unbounded
// sources by design — checking them would block legitimate flows.
func needsFundsCheck(t models.AccountType) bool {
	return t == models.AccountUserCash
}

// availableBalance translates raw net-debit balance into spendable units
// by flipping sign for CREDIT-normal account types. USER_CASH is a
// liability from the platform's POV, so a funded wallet has a negative
// raw balance.
func availableBalance(t models.AccountType, raw int64) int64 {
	if t == models.AccountUserCash {
		return -raw
	}
	return raw
}
