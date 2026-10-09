package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentOrderPayAmountPrecisionMigration(t *testing.T) {
	content, err := FS.ReadFile("9002_custom_payment_order_pay_amount_precision.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE payment_orders ALTER COLUMN pay_amount TYPE DECIMAL(21,3)")
	require.Contains(t, sql, "USING pay_amount::DECIMAL(21,3)")
	require.NotContains(t, sql, "ALTER COLUMN amount")
	require.NotContains(t, sql, "ALTER COLUMN refund_amount")
}
