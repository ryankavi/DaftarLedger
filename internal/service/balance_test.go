package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ryankavi/payclone/internal/models"
)

func TestNeedsFundsCheck(t *testing.T) {
	tests := []struct {
		at   models.AccountType
		want bool
	}{
		{models.AccountUserCash, true},
		{models.AccountExternal, false},
		{models.AccountTreasury, false},
		{models.AccountFeeRevenue, false},
		{models.AccountCardSettlement, false},
		{models.AccountACHClearing, false},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, needsFundsCheck(tc.at), "type=%s", tc.at)
	}
}

func TestAvailableBalance(t *testing.T) {
	// USER_CASH is CREDIT-normal: a funded wallet has a negative raw balance,
	// so availableBalance flips the sign. Other types pass through.
	tests := []struct {
		at   models.AccountType
		raw  int64
		want int64
	}{
		{models.AccountUserCash, -500, 500},
		{models.AccountUserCash, 0, 0},
		{models.AccountUserCash, 200, -200},
		{models.AccountExternal, 1000, 1000},
		{models.AccountTreasury, -300, -300},
		{models.AccountFeeRevenue, -250, -250},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, availableBalance(tc.at, tc.raw), "type=%s raw=%d", tc.at, tc.raw)
	}
}
