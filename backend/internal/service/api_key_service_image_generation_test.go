package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type imageOptionsAccountRepoStub struct {
	AccountRepository
	accounts []Account
	err      error
	calls    int
}

func (r *imageOptionsAccountRepoStub) ListByGroup(_ context.Context, _ int64) ([]Account, error) {
	r.calls++
	return r.accounts, r.err
}

type imageOptionsAPIKeyRepoStub struct {
	APIKeyRepository
	keys []APIKey
	err  error
}

func (r *imageOptionsAPIKeyRepoStub) ListByUserID(
	_ context.Context,
	_ int64,
	_ pagination.PaginationParams,
	_ APIKeyListFilters,
) ([]APIKey, *pagination.PaginationResult, error) {
	return r.keys, &pagination.PaginationResult{Total: int64(len(r.keys))}, r.err
}

func imageOptionsAccount(id int64, mapping map[string]any) Account {
	return Account{
		ID:          id,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"model_mapping": mapping},
	}
}

func TestImageGenerationModelsForAccountsRecognizesPublicAndMappedImageAliases(t *testing.T) {
	accounts := []Account{imageOptionsAccount(1, map[string]any{
		"gpt-image-1": "provider-art-v1",
		"artist-pro":  "gpt-image-2",
		"text-only":   "gpt-5.5",
		"gpt-image-*": "gpt-image-2",
	})}

	models := imageGenerationModelsForAccounts(&Group{Platform: PlatformOpenAI}, accounts)

	require.Equal(t, []string{"gpt-image-1", "artist-pro"}, models)
	require.NotContains(t, models, "gpt-image-2", "mapping targets are not public request aliases")
	require.NotContains(t, models, "gpt-image-*", "wildcards must never be exposed to the browser")
}

func TestImageGenerationModelsForAccountsUsesFiniteDefaultsForOpenMappings(t *testing.T) {
	accounts := []Account{imageOptionsAccount(1, nil)}

	models := imageGenerationModelsForAccounts(&Group{Platform: PlatformOpenAI}, accounts)

	require.Equal(t, []string{"gpt-image-2", "gpt-image-1.5", "gpt-image-1"}, models)
}

func TestImageGenerationModelsForAccountsHonorsCustomListAndDispatchability(t *testing.T) {
	group := &Group{
		Platform: PlatformOpenAI,
		ModelsListConfig: GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"studio", "qwen-image", "gpt-image-*", "studio"},
		},
	}
	accounts := []Account{
		imageOptionsAccount(1, map[string]any{
			"studio":       "gpt-image-2",
			"outside-list": "gpt-image-1",
		}),
		imageOptionsAccount(2, nil),
	}

	models := imageGenerationModelsForAccounts(group, accounts)

	require.Equal(t, []string{"qwen-image", "studio"}, models)
	require.NotContains(t, models, "outside-list")
	require.NotContains(t, models, "gpt-image-*")
}

func TestImageGenerationModelsForAccountsExcludesUnschedulableAndWrongPlatform(t *testing.T) {
	unschedulable := imageOptionsAccount(1, map[string]any{"gpt-image-2": "gpt-image-2"})
	unschedulable.Schedulable = false
	wrongPlatform := imageOptionsAccount(2, map[string]any{"gpt-image-1": "gpt-image-1"})
	wrongPlatform.Platform = PlatformAnthropic

	models := imageGenerationModelsForAccounts(
		&Group{Platform: PlatformOpenAI},
		[]Account{unschedulable, wrongPlatform},
	)

	require.Empty(t, models)
}

func TestImageGenerationModelsForGroupPropagatesRepositoryError(t *testing.T) {
	repo := &imageOptionsAccountRepoStub{err: errors.New("database unavailable")}
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	svc.SetAccountRepository(repo)

	models, err := svc.imageGenerationModelsForGroup(context.Background(), &Group{ID: 42})

	require.Nil(t, models)
	require.ErrorContains(t, err, "list group accounts")
}

func TestProvideAPIKeyServiceWiresPeterAIImageDependencies(t *testing.T) {
	accountRepo := &imageOptionsAccountRepoStub{}
	billingService := NewBillingService(&config.Config{}, nil)

	svc := ProvideAPIKeyService(
		nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil,
		accountRepo, billingService,
	)

	require.Same(t, accountRepo, svc.accountRepo)
	require.Same(t, billingService, svc.billingService)
}

func TestGetImageGenerationOptionsFiltersGroupsAndReturnsExplicitPrices(t *testing.T) {
	price1K, price2K, price4K := 0.11, 0.22, 0.44
	enabledGroup := &Group{
		ID:                        25,
		Name:                      "images",
		Platform:                  PlatformOpenAI,
		Status:                    StatusActive,
		AllowImageGeneration:      true,
		RateMultiplier:            1,
		ImageRateMultiplier:       1,
		ImagePrice1K:              &price1K,
		ImagePrice2K:              &price2K,
		ImagePrice4K:              &price4K,
		LongContextPricingEnabled: true,
	}
	disabledGroup := *enabledGroup
	disabledGroup.ID = 26
	disabledGroup.Name = "images-disabled"
	disabledGroup.AllowImageGeneration = false
	enabledID, disabledID := enabledGroup.ID, disabledGroup.ID
	keyRepo := &imageOptionsAPIKeyRepoStub{keys: []APIKey{
		{ID: 1, UserID: 7, Key: "sk-enabled", Name: "enabled", GroupID: &enabledID, Group: enabledGroup, Status: StatusActive},
		{ID: 2, UserID: 7, Key: "sk-disabled", Name: "disabled", GroupID: &disabledID, Group: &disabledGroup, Status: StatusActive},
	}}
	accountRepo := &imageOptionsAccountRepoStub{accounts: []Account{
		imageOptionsAccount(10, map[string]any{"gpt-image-2": "gpt-image-2"}),
	}}
	svc := NewAPIKeyService(keyRepo, nil, nil, nil, nil, nil, &config.Config{})
	svc.SetAccountRepository(accountRepo)
	svc.SetImagePricingDependencies(NewBillingService(&config.Config{}, nil), nil)

	options, err := svc.GetImageGenerationOptions(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, options.Keys, 1)
	require.Equal(t, "sk-enabled", options.Keys[0].Key)
	require.Equal(t, []string{"gpt-image-2"}, options.Keys[0].Models)
	require.Equal(t, price1K, *options.Keys[0].PricesByModel["gpt-image-2"]["1K"])
	require.Equal(t, price2K, *options.Keys[0].PricesByModel["gpt-image-2"]["2K"])
	require.Equal(t, price4K, *options.Keys[0].PricesByModel["gpt-image-2"]["4K"])
	require.Equal(t, 1, accountRepo.calls, "disabled groups must not trigger model discovery")
}

func TestGetImageGenerationOptionsFiltersExpiredAndExhaustedKeys(t *testing.T) {
	group := &Group{ID: 25, Platform: PlatformOpenAI, Status: StatusActive, AllowImageGeneration: true}
	groupID := group.ID
	expiredAt := time.Now().Add(-time.Minute)
	keyRepo := &imageOptionsAPIKeyRepoStub{keys: []APIKey{
		{ID: 1, GroupID: &groupID, Group: group, Status: StatusActive, ExpiresAt: &expiredAt},
		{ID: 2, GroupID: &groupID, Group: group, Status: StatusActive, Quota: 1, QuotaUsed: 1},
	}}
	accountRepo := &imageOptionsAccountRepoStub{accounts: []Account{
		imageOptionsAccount(10, map[string]any{"gpt-image-2": "gpt-image-2"}),
	}}
	svc := NewAPIKeyService(keyRepo, nil, nil, nil, nil, nil, &config.Config{})
	svc.SetAccountRepository(accountRepo)

	options, err := svc.GetImageGenerationOptions(context.Background(), 7)

	require.NoError(t, err)
	require.Empty(t, options.Keys)
	require.Zero(t, accountRepo.calls)
}
