//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestResolveRebateRatePercent_PerUserOverride verifies that per-inviter
// AffRebateRatePercent overrides the global rate, that NULL falls back to the
// global rate, and that out-of-range exclusive rates are clamped silently.
//
// SettingService is left nil here so globalRebateRatePercent returns the
// documented default (AffiliateRebateRateDefault = 20%) — this exercises the
// fallback path without spinning up a settings stub.
func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	// nil exclusive rate → falls back to global default (20%)
	require.InDelta(t, AffiliateRebateRateDefault,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}), 1e-9)

	// exclusive rate set → overrides global
	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	// exclusive rate 0 → returns 0 (no rebate, intentional)
	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	// exclusive rate above max → clamped to Max
	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	// exclusive rate below min → clamped to Min
	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestAccrueInviteRebatesForPaymentOrder_DifferentialTwoLevels(t *testing.T) {
	t.Parallel()
	repo := newAffiliateHierarchyServiceRepoStub(map[int64]*AffiliateSummary{
		1: affiliateSummaryForTest(1, nil, 20),
		2: affiliateSummaryForTest(2, affiliateTestInt64Ptr(1), 12),
		3: affiliateSummaryForTest(3, affiliateTestInt64Ptr(2), 0),
	})
	svc := NewAffiliateService(repo, newAffiliateHierarchySettingService(), nil, nil)

	result, err := svc.AccrueInviteRebatesForPaymentOrder(context.Background(), 3, 100, affiliateTestInt64Ptr(9001))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.InDelta(t, 20, result.TotalRebate, 1e-9)
	require.Len(t, result.Payouts, 2)
	require.Equal(t, int64(2), result.Payouts[0].UserID)
	require.InDelta(t, 12, result.Payouts[0].Amount, 1e-9)
	require.InDelta(t, 12, result.Payouts[0].RebateRatePercent, 1e-9)
	require.Equal(t, int64(1), result.Payouts[1].UserID)
	require.InDelta(t, 8, result.Payouts[1].Amount, 1e-9)
	require.InDelta(t, 8, result.Payouts[1].RebateRatePercent, 1e-9)
	require.Len(t, repo.payouts, 2)
}

func TestAccrueInviteRebatesForPaymentOrder_DifferentialThreeLevels(t *testing.T) {
	t.Parallel()
	repo := newAffiliateHierarchyServiceRepoStub(map[int64]*AffiliateSummary{
		1: affiliateSummaryForTest(1, nil, 20),
		2: affiliateSummaryForTest(2, affiliateTestInt64Ptr(1), 12),
		3: affiliateSummaryForTest(3, affiliateTestInt64Ptr(2), 8),
		4: affiliateSummaryForTest(4, affiliateTestInt64Ptr(3), 0),
	})
	svc := NewAffiliateService(repo, newAffiliateHierarchySettingService(), nil, nil)

	result, err := svc.AccrueInviteRebatesForPaymentOrder(context.Background(), 4, 100, affiliateTestInt64Ptr(9002))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.InDelta(t, 20, result.TotalRebate, 1e-9)
	require.Len(t, result.Payouts, 3)
	require.Equal(t, int64(3), result.Payouts[0].UserID)
	require.InDelta(t, 8, result.Payouts[0].Amount, 1e-9)
	require.Equal(t, int64(2), result.Payouts[1].UserID)
	require.InDelta(t, 4, result.Payouts[1].Amount, 1e-9)
	require.Equal(t, int64(1), result.Payouts[2].UserID)
	require.InDelta(t, 8, result.Payouts[2].Amount, 1e-9)
	require.Len(t, repo.payouts, 3)
}

func TestGetMyAffiliateHierarchyRequiresAgentAccess(t *testing.T) {
	t.Parallel()
	repo := newAffiliateHierarchyServiceRepoStub(map[int64]*AffiliateSummary{
		10: affiliateSummaryForTest(10, nil, 20),
	})
	repo.access = map[int64]*AffiliateAgentAccess{
		10: {UserID: 10, Enabled: false},
	}
	svc := NewAffiliateService(repo, newAffiliateHierarchySettingService(), nil, nil)

	report, err := svc.GetMyAffiliateHierarchy(context.Background(), 10, AffiliateHierarchyFilter{RootUserID: 999})
	require.Nil(t, report)
	require.ErrorIs(t, err, ErrAgentAccessDenied)
	require.Nil(t, repo.hierarchyFilter)
}

func TestGetMyAffiliateHierarchyForcesCurrentUserRoot(t *testing.T) {
	t.Parallel()
	repo := newAffiliateHierarchyServiceRepoStub(map[int64]*AffiliateSummary{
		10: affiliateSummaryForTest(10, nil, 20),
	})
	repo.access = map[int64]*AffiliateAgentAccess{
		10: {UserID: 10, Enabled: true},
	}
	svc := NewAffiliateService(repo, newAffiliateHierarchySettingService(), nil, nil)

	report, err := svc.GetMyAffiliateHierarchy(context.Background(), 10, AffiliateHierarchyFilter{
		RootUserID: 999,
		Search:     " child ",
		MaxDepth:   3,
		Limit:      25,
	})
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Equal(t, int64(10), report.Summary.RootUserID)
	require.NotNil(t, repo.hierarchyFilter)
	require.Equal(t, int64(10), repo.hierarchyFilter.RootUserID)
	require.Equal(t, "child", repo.hierarchyFilter.Search)
	require.Equal(t, 3, repo.hierarchyFilter.MaxDepth)
	require.Equal(t, 25, repo.hierarchyFilter.Limit)
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}

type affiliateHierarchyServiceRepoStub struct {
	summaries       map[int64]*AffiliateSummary
	payouts         []AffiliateRebatePayout
	access          map[int64]*AffiliateAgentAccess
	hierarchyFilter *AffiliateHierarchyFilter
}

func newAffiliateHierarchyServiceRepoStub(summaries map[int64]*AffiliateSummary) *affiliateHierarchyServiceRepoStub {
	return &affiliateHierarchyServiceRepoStub{summaries: summaries}
}

func (r *affiliateHierarchyServiceRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	summary, ok := r.summaries[userID]
	if !ok {
		return nil, ErrAffiliateProfileNotFound
	}
	cp := *summary
	if summary.InviterID != nil {
		inviterID := *summary.InviterID
		cp.InviterID = &inviterID
	}
	if summary.AffRebateRatePercent != nil {
		rate := *summary.AffRebateRatePercent
		cp.AffRebateRatePercent = &rate
	}
	return &cp, nil
}

func (r *affiliateHierarchyServiceRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	panic("unexpected GetAffiliateByCode call")
}

func (r *affiliateHierarchyServiceRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	panic("unexpected BindInviter call")
}

func (r *affiliateHierarchyServiceRepoStub) AccrueQuota(context.Context, int64, int64, float64, int, *int64) (bool, error) {
	panic("unexpected AccrueQuota call")
}

func (r *affiliateHierarchyServiceRepoStub) AccrueHierarchicalQuota(_ context.Context, payouts []AffiliateRebatePayout, _ int) (int, error) {
	r.payouts = append(r.payouts, payouts...)
	return len(payouts), nil
}

func (r *affiliateHierarchyServiceRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}

func (r *affiliateHierarchyServiceRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	panic("unexpected ThawFrozenQuota call")
}

func (r *affiliateHierarchyServiceRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (r *affiliateHierarchyServiceRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	panic("unexpected ListInvitees call")
}

func (r *affiliateHierarchyServiceRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (r *affiliateHierarchyServiceRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (r *affiliateHierarchyServiceRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (r *affiliateHierarchyServiceRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (r *affiliateHierarchyServiceRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func (r *affiliateHierarchyServiceRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	panic("unexpected ListAffiliateInviteRecords call")
}

func (r *affiliateHierarchyServiceRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	panic("unexpected ListAffiliateRebateRecords call")
}

func (r *affiliateHierarchyServiceRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	panic("unexpected ListAffiliateTransferRecords call")
}

func (r *affiliateHierarchyServiceRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	panic("unexpected GetAffiliateUserOverview call")
}

func (r *affiliateHierarchyServiceRepoStub) GetMaxDirectChildRebateRate(context.Context, int64, float64) (float64, bool, error) {
	panic("unexpected GetMaxDirectChildRebateRate call")
}

func (r *affiliateHierarchyServiceRepoStub) ListAffiliateHierarchyRoots(context.Context, AffiliateHierarchyRootFilter, float64) ([]AffiliateHierarchyRoot, error) {
	panic("unexpected ListAffiliateHierarchyRoots call")
}

func (r *affiliateHierarchyServiceRepoStub) GetAffiliateHierarchy(_ context.Context, filter AffiliateHierarchyFilter, _ float64) (*AffiliateHierarchyReport, error) {
	copied := filter
	r.hierarchyFilter = &copied
	return &AffiliateHierarchyReport{
		Summary: AffiliateHierarchySummary{RootUserID: filter.RootUserID, NodeCount: 1},
		Nodes: []AffiliateHierarchyNode{
			{UserID: filter.RootUserID, Depth: 0, Path: []int64{filter.RootUserID}},
		},
	}, nil
}

func (r *affiliateHierarchyServiceRepoStub) GetAgentAccess(_ context.Context, userID int64, globalRate float64) (*AffiliateAgentAccess, error) {
	if r.access != nil {
		if access, ok := r.access[userID]; ok {
			cp := *access
			return &cp, nil
		}
	}
	summary, ok := r.summaries[userID]
	if !ok {
		return nil, ErrAffiliateProfileNotFound
	}
	return &AffiliateAgentAccess{
		UserID:                     userID,
		AffCode:                    summary.AffCode,
		EffectiveRebateRatePercent: globalRate,
		Enabled:                    false,
	}, nil
}

func (r *affiliateHierarchyServiceRepoStub) SetAgentAccess(context.Context, int64, bool, string, int64) error {
	panic("unexpected SetAgentAccess call")
}

type affiliateHierarchySettingRepoStub struct {
	values map[string]string
}

func newAffiliateHierarchySettingService() *SettingService {
	return NewSettingService(&affiliateHierarchySettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:           "true",
		SettingKeyAffiliateRebateRate:        "20",
		SettingKeyAffiliateRebateFreezeHours: "0",
	}}, nil)
}

func (r *affiliateHierarchySettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *affiliateHierarchySettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *affiliateHierarchySettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *affiliateHierarchySettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r *affiliateHierarchySettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *affiliateHierarchySettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *affiliateHierarchySettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func affiliateSummaryForTest(userID int64, inviterID *int64, rate float64) *AffiliateSummary {
	rateCopy := rate
	return &AffiliateSummary{
		UserID:               userID,
		InviterID:            inviterID,
		AffRebateRatePercent: &rateCopy,
		CreatedAt:            time.Now().Add(-time.Hour),
	}
}

func affiliateTestInt64Ptr(v int64) *int64 {
	return &v
}

var _ AffiliateRepository = (*affiliateHierarchyServiceRepoStub)(nil)
var _ SettingRepository = (*affiliateHierarchySettingRepoStub)(nil)
