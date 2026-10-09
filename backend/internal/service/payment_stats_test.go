//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	appTimezone "github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestComputeBasicStatsGroupsAmountsByCurrency(t *testing.T) {
	t.Parallel()

	todayStart := time.Date(2026, time.July, 25, 0, 0, 0, 0, time.UTC)
	yesterday := todayStart.Add(-time.Hour)
	today := todayStart.Add(time.Hour)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 10, &today),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 10, &today),
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 5, &yesterday),
	}

	stats := &DashboardStats{}
	computeBasicStats(stats, orders, todayStart)

	require.Equal(t, CurrencyAmounts{"CNY": 15, "USD": 10}, stats.TotalAmount)
	require.Equal(t, CurrencyAmounts{"CNY": 10, "USD": 10}, stats.TodayAmount)
	require.Equal(t, CurrencyAmounts{"CNY": 7.5, "USD": 10}, stats.AvgAmount)
	require.Equal(t, 3, stats.TotalCount)
	require.Equal(t, 2, stats.TodayCount)
}

func TestPaymentDashboardBreakdownsGroupAmountsAndRankingsByCurrency(t *testing.T) {
	t.Parallel()

	firstDay := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	secondDay := firstDay.AddDate(0, 0, 1)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 5.555, &firstDay),
		paymentStatsTestOrder(2, "bob@example.com", "CNY", 10, &firstDay),
		paymentStatsTestOrder(1, "alice@example.com", "USD", 20, &secondDay),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 10, &secondDay),
	}
	orders[0].PaymentType = "stripe"
	orders[1].PaymentType = "stripe"
	orders[2].PaymentType = "stripe"
	orders[3].PaymentType = "alipay"

	daily := buildDailySeries(orders, firstDay.AddDate(0, 0, -1), 2)
	require.Equal(t, []DailyStats{
		{Date: "2026-07-24", Amount: CurrencyAmounts{"CNY": 15.56}, Count: 2},
		{Date: "2026-07-25", Amount: CurrencyAmounts{"USD": 30}, Count: 2},
	}, daily)

	methods := buildMethodDistribution(orders)
	require.Equal(t, []PaymentMethodStat{
		{Type: "alipay", Amount: CurrencyAmounts{"USD": 10}, Count: 1},
		{Type: "stripe", Amount: CurrencyAmounts{"CNY": 15.56, "USD": 20}, Count: 3},
	}, methods)

	users := buildTopUsers(orders)
	require.Equal(t, TopUsersByCurrency{
		"CNY": {
			{UserID: 2, Email: "bob@example.com", Amount: 10},
			{UserID: 1, Email: "alice@example.com", Amount: 5.56},
		},
		"USD": {
			{UserID: 1, Email: "alice@example.com", Amount: 20},
			{UserID: 2, Email: "bob@example.com", Amount: 10},
		},
	}, users)
}

func TestBuildMonthlySeriesUsesCalendarMonthsAndSeparatesCurrencies(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2025, time.December, 1, 0, 0, 0, 0, loc)
	december := time.Date(2025, time.December, 31, 23, 0, 0, 0, loc)
	february := time.Date(2026, time.February, 10, 12, 0, 0, 0, loc)
	orders := []*dbent.PaymentOrder{
		nil,
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 10.125, &december),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 20, &december),
		paymentStatsTestOrder(3, "carol@example.com", "CNY", 5, &february),
		paymentStatsTestOrder(4, "nobody@example.com", "USD", 999, nil),
		paymentStatsTestOrder(5, "kwd@example.com", "KWD", 12.345, &february),
		paymentStatsTestOrder(6, "jpy@example.com", "JPY", 321, &february),
	}

	series := buildMonthlySeries(orders, start, 3, loc)

	require.Equal(t, []MonthlyStats{
		{Month: "2025-12", Amount: CurrencyAmounts{"CNY": 10.13, "USD": 20}, Count: 2},
		{Month: "2026-01", Amount: CurrencyAmounts{}, Count: 0},
		{Month: "2026-02", Amount: CurrencyAmounts{"CNY": 5, "JPY": 321, "KWD": 12.345}, Count: 3},
	}, series)
}

func TestRoundCurrencyAmountsByCurrencyUsesCurrencyPrecision(t *testing.T) {
	t.Parallel()

	amounts := CurrencyAmounts{"USD": 1.235, "KWD": 12.3454, "JPY": 100.6}
	roundCurrencyAmountsByCurrency(amounts)

	require.Equal(t, CurrencyAmounts{"USD": 1.24, "KWD": 12.345, "JPY": 101}, amounts)
}

func TestBuildMonthlySeriesUsesBusinessTimezoneAtMonthBoundary(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, loc)
	// 16:30 UTC is already February 1 in Asia/Shanghai.
	paidAt := time.Date(2026, time.January, 31, 16, 30, 0, 0, time.UTC)
	order := paymentStatsTestOrder(1, "alice@example.com", "CNY", 12, &paidAt)
	order.Amount = 999
	order.BonusAmount = 100
	order.Status = OrderStatusRefunded

	series := buildMonthlySeries([]*dbent.PaymentOrder{order}, start, 2, loc)

	require.Equal(t, MonthlyStats{Month: "2026-01", Amount: CurrencyAmounts{}, Count: 0}, series[0])
	require.Equal(t, MonthlyStats{Month: "2026-02", Amount: CurrencyAmounts{"CNY": 12}, Count: 1}, series[1])
}

func TestBuildMonthlySeriesRejectsInvalidRangeAndOutOfRangeOrders(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	before := start.Add(-time.Second)
	after := start.AddDate(0, 2, 0)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "before@example.com", "USD", 10, &before),
		paymentStatsTestOrder(2, "after@example.com", "USD", 20, &after),
	}

	require.Empty(t, buildMonthlySeries(orders, start, 0, time.UTC))
	require.Equal(t, []MonthlyStats{
		{Month: "2026-03", Amount: CurrencyAmounts{}, Count: 0},
		{Month: "2026-04", Amount: CurrencyAmounts{}, Count: 0},
	}, buildMonthlySeries(orders, start, 2, time.UTC))
}

func TestGetDashboardStatsReturnsFixedMonthlyGrossReceipts(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:payment_stats_dashboard?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	user, err := client.User.Create().
		SetEmail("monthly@example.com").
		SetPasswordHash("hash").
		SetUsername("monthly").
		Save(ctx)
	require.NoError(t, err)

	now := appTimezone.Now()
	currentPaidAt := now.Add(-time.Hour)
	historicalPaidAt := now.AddDate(0, -2, 0)
	futurePaidAt := appTimezone.StartOfMonth(now).AddDate(0, 1, 0).Add(-time.Nanosecond)
	if !futurePaidAt.After(now) {
		t.Skip("cannot construct a future paid_at within the current calendar month")
	}
	createOrder := func(outTradeNo, status, currency string, payAmount float64, paidAt *time.Time) {
		builder := client.PaymentOrder.Create().
			SetUserID(user.ID).
			SetUserEmail(user.Email).
			SetUserName(user.Username).
			SetAmount(payAmount).
			SetPayAmount(payAmount).
			SetRechargeCode("MONTHLY-" + outTradeNo).
			SetOutTradeNo(outTradeNo).
			SetPaymentType(payment.TypeStripe).
			SetPaymentTradeNo("trade-" + outTradeNo).
			SetOrderType(payment.OrderTypeBalance).
			SetStatus(status).
			SetProviderSnapshot(map[string]any{"currency": currency}).
			SetExpiresAt(now.Add(time.Hour)).
			SetClientIP("127.0.0.1").
			SetSrcHost("api.example.com")
		if paidAt != nil {
			builder.SetPaidAt(*paidAt)
		}
		_, saveErr := builder.Save(ctx)
		require.NoError(t, saveErr)
	}

	createOrder("monthly-current", OrderStatusCompleted, "USD", 12.34, &currentPaidAt)
	createOrder("monthly-refunded", OrderStatusRefunded, "CNY", 42, &historicalPaidAt)
	createOrder("monthly-pending", OrderStatusPending, "USD", 99, nil)
	createOrder("monthly-future", OrderStatusCompleted, "USD", 88, &futurePaidAt)

	stats, err := (&PaymentService{entClient: client}).GetDashboardStats(ctx, 7)
	require.NoError(t, err)
	require.Len(t, stats.MonthlySeries, monthlyRevenueMonths)
	require.Equal(t, 1, stats.PendingOrders)

	monthlyByKey := make(map[string]MonthlyStats, len(stats.MonthlySeries))
	for _, item := range stats.MonthlySeries {
		monthlyByKey[item.Month] = item
	}
	require.Equal(t, CurrencyAmounts{"USD": 12.34}, monthlyByKey[currentPaidAt.In(appTimezone.Location()).Format("2006-01")].Amount)
	require.Equal(t, CurrencyAmounts{"CNY": 42}, monthlyByKey[historicalPaidAt.In(appTimezone.Location()).Format("2006-01")].Amount)
}

func paymentStatsTestOrder(userID int64, email, currency string, amount float64, paidAt *time.Time) *dbent.PaymentOrder {
	return &dbent.PaymentOrder{
		UserID:           userID,
		UserEmail:        email,
		PayAmount:        amount,
		PaidAt:           paidAt,
		ProviderSnapshot: map[string]any{"currency": currency},
	}
}
