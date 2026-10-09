//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestPaymentOrderPayAmountPreservesThreeFractionDigits(t *testing.T) {
	ctx := context.Background()

	var precision, scale int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT numeric_precision, numeric_scale
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'payment_orders'
  AND column_name = 'pay_amount'
`).Scan(&precision, &scale))
	require.Equal(t, 21, precision)
	require.Equal(t, 3, scale)

	tx := testEntTx(t)
	client := tx.Client()
	user, err := client.User.Create().
		SetEmail("payment-precision@example.com").
		SetPasswordHash("hash").
		SetUsername("payment-precision").
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(10).
		SetPayAmount(12.345).
		SetRechargeCode("PAYMENT-PRECISION").
		SetOutTradeNo("payment-precision-kwd").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("trade-payment-precision").
		SetOrderType(payment.OrderTypeBalance).
		SetProviderSnapshot(map[string]any{"currency": "KWD"}).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	stored, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.345, stored.PayAmount, 0.000001)
}
