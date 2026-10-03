package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParseOpenAIManagedMediaRequest(t *testing.T) {
	audio, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaAudioSpeech, []byte(`{"model":"gpt-4o-mini-tts","input":"你好"}`))
	require.NoError(t, err)
	require.Equal(t, "gpt-4o-mini-tts", audio.Model)

	seedance, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, []byte(`{"model":"doubao-seedance-2-0-pro","content":[{"type":"text","text":"海浪"}],"resolution":"1080p","duration":12}`))
	require.NoError(t, err)
	require.Equal(t, "1080p", seedance.Resolution)
	require.Equal(t, 12, seedance.DurationSeconds)
	require.False(t, seedance.DurationAuto)

	autoSeedance, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, []byte(`{"model":"doubao-seedance-2-0-pro","content":[{"type":"text","text":"海浪"}],"resolution":"1080p"}`))
	require.NoError(t, err)
	require.True(t, autoSeedance.DurationAuto)
	require.Equal(t, VideoBillingDefaultDurationSeconds, autoSeedance.DurationSeconds)

	canonicalSeedance, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, []byte(`{"model":"doubao-seedance-2-0-pro","resolution":" FHD ","duration":"12"}`))
	require.NoError(t, err)
	require.Equal(t, VideoBillingResolution1080P, canonicalSeedance.Resolution)
	require.Equal(t, 12, canonicalSeedance.DurationSeconds)
	require.False(t, canonicalSeedance.DurationAuto)

	for _, invalidBody := range []string{
		`{"model":123,"resolution":"1080p","duration":12}`,
		`{"model":"doubao-seedance-2-0-pro","resolution":"4k","duration":12}`,
		`{"model":"doubao-seedance-2-0-pro","resolution":1080,"duration":12}`,
		`{"model":"doubao-seedance-2-0-pro","resolution":"1080p","duration":16}`,
		`{"model":"doubao-seedance-2-0-pro","resolution":"1080p","duration":12.5}`,
		`{"model":"doubao-seedance-2-0-pro","resolution":"1080p","duration":0}`,
	} {
		_, parseErr := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, []byte(invalidBody))
		require.Error(t, parseErr, invalidBody)
	}

	for _, ambiguousBody := range []string{
		`{"model":"seedance","duration":1,"duration":15}`,
		`{"model":"seedance","resolution":"480p","resolution":"1080p"}`,
		`{"model":"seedance","duration":1,"dur\u0061tion":15}`,
		`{"model":"seedance","duration":1,"Duration":15}`,
		`{"model":"seedance","content":[{"type":"text","text":"safe","text":"other"}]}`,
	} {
		_, parseErr := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, []byte(ambiguousBody))
		require.ErrorContains(t, parseErr, "JSON field", ambiguousBody)
	}

	for _, nonCanonicalModerationBody := range []string{
		`{"model":"seedance","Content":[{"type":"text","text":"unsafe"}]}`,
		`{"model":"seedance","content":[{"Type":"text","text":"unsafe"}]}`,
		`{"model":"seedance","content":[{"type":"text","Text":"unsafe"}]}`,
		`{"model":"seedance","content":[{"type":"image_url","Image_URL":{"url":"https://example.com/unsafe.png"}}]}`,
		`{"model":"seedance","content":[{"type":"image_url","image_url":{"URL":"https://example.com/unsafe.png"}}]}`,
	} {
		_, parseErr := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, []byte(nonCanonicalModerationBody))
		require.ErrorContains(t, parseErr, "canonical lowercase", nonCanonicalModerationBody)
	}

	_, err = ParseOpenAIManagedMediaRequest(OpenAIManagedMediaAudioSpeech, []byte(`{"model":"gpt-4o-mini-tts"}`))
	require.ErrorContains(t, err, "input must be")
}

func TestOpenAIManagedMediaModerationBody(t *testing.T) {
	audio := OpenAIManagedMediaModerationBody(OpenAIManagedMediaAudioSpeech, []byte(`{"model":"tts","input":"hello"}`))
	require.Equal(t, "hello", gjson.GetBytes(audio, "prompt").String())

	seedance := OpenAIManagedMediaModerationBody(OpenAIManagedMediaSeedanceCreate, []byte(`{"model":"seedance","content":[{"type":"text","text":"waves"},{"type":"image_url","image_url":{"url":"https://example.com/ref.png"}},{"type":"video_url","video_url":{"url":"https://example.com/ref.mp4"}}]}`))
	require.Equal(t, "waves", gjson.GetBytes(seedance, "prompt").String())
	require.Equal(t, "https://example.com/ref.png", gjson.GetBytes(seedance, "images.0.image_url").String())
	require.Len(t, gjson.GetBytes(seedance, "images").Array(), 1)
}

func TestForwardOpenAIManagedMediaAudioSpeech(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"tts-public","input":"hello","voice":"alloy"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"audio/mpeg"}, "X-Request-Id": []string{"speech-1"}},
		Body:       io.NopCloser(strings.NewReader("audio-bytes")),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "sk-test", "base_url": "https://speech.example/v1", "openai_capabilities": []any{"audio_speech"},
		"model_mapping": map[string]any{"tts-channel": "tts-deployment"},
	}}
	parsed, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaAudioSpeech, body)
	require.NoError(t, err)

	result, err := svc.ForwardOpenAIManagedMedia(context.Background(), c, account, OpenAIManagedMediaAudioSpeech, "", body, parsed, "tts-channel")

	require.NoError(t, err)
	require.Equal(t, "https://speech.example/v1/audio/speech", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "tts-deployment", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "audio/mpeg", recorder.Header().Get("Content-Type"))
	require.Equal(t, "audio-bytes", recorder.Body.String())
	require.Equal(t, 1, result.RequestCount)
	require.Equal(t, "audio", result.MediaType)
	require.Equal(t, "speech-1", result.RequestID)
	require.Equal(t, "tts-channel", result.BillingModel)
	require.Equal(t, "tts-deployment", result.UpstreamModel)
	opsModel, ok := c.Get(OpsUpstreamModelKey)
	require.True(t, ok)
	require.Equal(t, "tts-deployment", opsModel)
}

func TestRecordOpenAIManagedAudioUpstreamBillingSourceUsesAccountModel(t *testing.T) {
	groupID := int64(75)
	requestedModel := "tts-public"
	channelModel := "tts-channel"
	accountModel := "tts-account"
	requestedPrice := 0.02
	channelPrice := 0.04
	accountPrice := 0.08
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	cache := newEmptyChannelCache()
	for model, price := range map[string]*float64{
		requestedModel: &requestedPrice,
		channelModel:   &channelPrice,
		accountModel:   &accountPrice,
	} {
		cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: model}] = &ChannelModelPricing{
			BillingMode: BillingModePerRequest, PerRequestPrice: price,
		}
	}
	cache.channelByGroupID[groupID] = &Channel{ID: 1, Status: StatusActive}
	cache.groupPlatform[groupID] = PlatformOpenAI
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	svc.channelService = channelService
	svc.resolver = NewModelPricingResolver(channelService, svc.billingService)

	group := &Group{ID: groupID, Platform: PlatformOpenAI, RateMultiplier: 2}
	user := &User{ID: 175}
	apiKey := &APIKey{ID: 275, UserID: user.ID, User: user, GroupID: &groupID, Group: group}
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "managed_audio_upstream_source",
			Model:         requestedModel,
			BillingModel:  accountModel,
			UpstreamModel: accountModel,
			RequestCount:  1,
			MediaType:     "audio",
			Duration:      time.Second,
		},
		APIKey:  apiKey,
		User:    user,
		Account: &Account{ID: 375, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      requestedModel,
			ChannelMappedModel: channelModel,
			BillingModelSource: BillingModelSourceUpstream,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, 0.08, usageRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 0.16, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, 0.16, userRepo.lastAmount, 1e-12)
}

func TestForwardOpenAIManagedMediaSeedanceUsesArkVersionedBase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":" seedance-public ","content":[{"type":"text","text":"waves"}],"resolution":"720p"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/contents/generations/tasks", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"task-123","status":"queued"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "ark-test", "base_url": "https://ark.example/api/plan/v3", "openai_capabilities": []any{"seedance"},
	}}
	parsed, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, body)
	require.NoError(t, err)

	result, err := svc.ForwardOpenAIManagedMedia(context.Background(), c, account, OpenAIManagedMediaSeedanceCreate, "", body, parsed, "")

	require.NoError(t, err)
	require.Equal(t, "https://ark.example/api/plan/v3/contents/generations/tasks", upstream.lastReq.URL.String())
	require.Equal(t, "seedance-public", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "720p", gjson.GetBytes(upstream.lastBody, "resolution").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "duration").Exists(), "automatic duration must be omitted upstream")
	require.Equal(t, "task-123", result.ResponseID)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, "720p", result.VideoResolution)
	require.Equal(t, VideoBillingDefaultDurationSeconds, result.VideoDurationSeconds)
	require.Equal(t, 1, result.RequestCount)
	require.Equal(t, "video", result.MediaType)
}

func TestForwardOpenAIManagedMediaSeedanceRejectsMissingTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"seedance","content":[{"type":"text","text":"waves"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/contents/generations/tasks", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"status":"queued"}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 44, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "x", "base_url": "https://ark.example/api/plan/v3", "openai_capabilities": []any{"seedance"}}}
	parsed, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaSeedanceCreate, body)
	require.NoError(t, err)

	_, err = svc.ForwardOpenAIManagedMedia(context.Background(), c, account, OpenAIManagedMediaSeedanceCreate, "", body, parsed, "")

	require.ErrorContains(t, err, "task ID")
	require.Empty(t, recorder.Body.String())
}

func TestValidateOpenAIManagedMediaPricingRequiresExplicitPeterPrice(t *testing.T) {
	groupID := int64(7)
	channel := &Channel{ID: 3, Status: StatusActive}
	channelService := NewChannelService(nil, nil, nil, nil, nil)
	cache := &channelCache{
		pricingByGroupModel:     map[channelModelKey]*ChannelModelPricing{},
		wildcardByGroupPlatform: map[channelGroupPlatformKey][]*wildcardPricingEntry{},
		mappingByGroupModel:     map[channelModelKey]string{},
		wildcardMappingByGP:     map[channelGroupPlatformKey][]*wildcardMappingEntry{},
		channelByGroupID:        map[int64]*Channel{groupID: channel},
		groupPlatform:           map[int64]string{groupID: PlatformOpenAI},
		loadedAt:                time.Now(),
	}
	channelService.cache.Store(cache)
	svc := &OpenAIGatewayService{channelService: channelService}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformOpenAI}}

	err := svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaAudioSpeech, "tts", "", "", false)
	require.Error(t, err)

	freePrice := 0.0
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: "tts"}] = &ChannelModelPricing{
		BillingMode: BillingModePerRequest, PerRequestPrice: &freePrice,
	}
	require.NoError(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaAudioSpeech, "tts", "", "", false))
	paidPrice := 0.04
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: "tts"}].PerRequestPrice = &paidPrice
	billingService := NewBillingService(&config.Config{}, nil)
	svc.billingService = billingService
	svc.resolver = NewModelPricingResolver(channelService, billingService)
	cost, err := svc.calculateOpenAIRequestCost(context.Background(), []string{"missing-model", "tts"}, apiKey, 1, "", 2)
	require.NoError(t, err)
	require.InDelta(t, 0.04, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.08, cost.ActualCost, 1e-9)

	price720 := 0.1
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: "seedance"}] = &ChannelModelPricing{
		BillingMode: BillingModePerRequest,
		Intervals:   []PricingInterval{{TierLabel: " 720P ", PerRequestPrice: &price720}},
	}
	require.NoError(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "720p", true))
	require.Error(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "1080p", false))

	price1080 := 0.2
	apiKey.Group.VideoPrice1080P = &price1080
	require.NoError(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "1080p", false))
	require.Error(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "1080p", true))
}

func TestRecordOpenAIManagedAudioUsesPerRequestPricing(t *testing.T) {
	groupID := int64(71)
	model := "peterai-tts"
	price := 0.04
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	resolver := newOpenAIImageChannelPricingResolverForTest(t, groupID, model, price)
	cache, ok := resolver.channelService.cache.Load().(*channelCache)
	require.True(t, ok)
	pricing, ok := cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}]
	require.True(t, ok)
	pricing.BillingMode = BillingModePerRequest
	svc.channelService = resolver.channelService
	svc.resolver = resolver

	group := &Group{
		ID:                 groupID,
		Platform:           PlatformOpenAI,
		SubscriptionType:   SubscriptionTypeSubscription,
		RateMultiplier:     2,
		PeakRateEnabled:    true,
		PeakStart:          "00:00",
		PeakEnd:            "23:59",
		PeakRateMultiplier: 3,
	}
	user := &User{ID: 171}
	apiKey := &APIKey{ID: 271, UserID: user.ID, User: user, GroupID: &groupID, Group: group}
	estimate, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{
		Model: model, Kind: InflightEstimateAudio, AudioMode: "tts", AudioUnits: 1,
		RequireChannelPerRequest: true,
	})
	require.True(t, priced)
	require.InDelta(t, 0.08, estimate, 1e-12, "managed audio reservation must use the explicit per-request price")
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:    "managed_audio_per_request",
			Model:        model,
			BillingModel: model,
			RequestCount: 1,
			MediaType:    "audio",
			Duration:     time.Second,
		},
		APIKey:    apiKey,
		User:      user,
		Account:   &Account{ID: 371, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		PricingAt: time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, 0.04, usageRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 0.08, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, 0.08, userRepo.lastAmount, 1e-12)
	require.InDelta(t, 2.0, usageRepo.lastLog.RateMultiplier, 1e-12,
		"managed audio must use the base multiplier rather than the token peak multiplier")
	require.NotNil(t, usageRepo.lastLog.BillingMode)
	require.Equal(t, string(BillingModePerRequest), *usageRepo.lastLog.BillingMode)
}

func TestRecordAutoDurationSeedanceUsesTieredPerRequestPrice(t *testing.T) {
	groupID := int64(72)
	model := "peterai-seedance"
	tierPrice := 0.20
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	resolver := newOpenAIImageChannelPricingResolverForTest(t, groupID, model, tierPrice)
	cache, ok := resolver.channelService.cache.Load().(*channelCache)
	require.True(t, ok)
	pricing, ok := cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}]
	require.True(t, ok)
	pricing.BillingMode = BillingModePerRequest
	pricing.PerRequestPrice = nil
	pricing.Intervals = []PricingInterval{{TierLabel: " 1080P ", PerRequestPrice: &tierPrice}}
	svc.channelService = resolver.channelService
	svc.resolver = resolver

	legacyPerSecond := 1.0
	group := &Group{
		ID:                   groupID,
		Platform:             PlatformOpenAI,
		RateMultiplier:       2,
		VideoPrice1080P:      &legacyPerSecond,
		VideoRateIndependent: true,
		VideoRateMultiplier:  0.25,
		ModelPricing: []ChannelModelPricing{{
			Models:      []string{model},
			BillingMode: BillingModeVideo,
			Intervals:   []PricingInterval{{TierLabel: "1080p", PerRequestPrice: &legacyPerSecond}},
		}},
	}
	user := &User{ID: 172}
	apiKey := &APIKey{ID: 272, UserID: user.ID, User: user, GroupID: &groupID, Group: group}
	estimate, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{
		Model: model, Kind: InflightEstimatePerRequest, Units: 1,
		VideoResolution: "1080p", RequireChannelPerRequest: true, UseVideoRateMultiplier: true,
	})
	require.True(t, priced)
	require.InDelta(t, 0.05, estimate, 1e-12,
		"auto-duration reservation must use the channel request price and dedicated video multiplier")
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:            "managed_seedance_auto_per_request",
			Model:                model,
			BillingModel:         model,
			RequestCount:         1,
			VideoCount:           1,
			VideoResolution:      "1080p",
			VideoDurationSeconds: VideoBillingDefaultDurationSeconds,
			MediaType:            "video",
			Duration:             time.Second,
		},
		APIKey:  apiKey,
		User:    user,
		Account: &Account{ID: 372, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, 0.20, usageRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 0.05, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, 0.05, userRepo.lastAmount, 1e-12)
	require.InDelta(t, 0.25, usageRepo.lastLog.RateMultiplier, 1e-12)
	require.Nil(t, usageRepo.lastLog.VideoDurationSeconds, "auto duration must not be persisted as guessed billable seconds")
	require.NotNil(t, usageRepo.lastLog.BillingMode)
	require.Equal(t, string(BillingModePerRequest), *usageRepo.lastLog.BillingMode)
}

func TestRecordExplicitDurationSeedanceUsesPerSecondVideoPrice(t *testing.T) {
	groupID := int64(73)
	model := "peterai-seedance-explicit"
	mappedModel := "seedance-upstream-token"
	perSecond := 0.10
	inputPrice := 1e-6
	outputPrice := 2e-6
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	cache := newEmptyChannelCache()
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: mappedModel}] = &ChannelModelPricing{
		BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice,
	}
	cache.channelByGroupID[groupID] = &Channel{ID: 1, Status: StatusActive}
	cache.groupPlatform[groupID] = PlatformOpenAI
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	svc.channelService = channelService
	svc.resolver = NewModelPricingResolver(channelService, svc.billingService)

	group := &Group{
		ID:                   groupID,
		Platform:             PlatformOpenAI,
		RateMultiplier:       2,
		VideoRateIndependent: true,
		VideoRateMultiplier:  0.25,
		VideoModelPrices: map[string]map[string]float64{
			model: {VideoBillingResolution1080P: perSecond},
		},
	}
	user := &User{ID: 173}
	apiKey := &APIKey{ID: 273, UserID: user.ID, User: user, GroupID: &groupID, Group: group}
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:            "managed_seedance_explicit_video",
			Model:                model,
			BillingModel:         model,
			UpstreamModel:        mappedModel,
			VideoCount:           1,
			VideoResolution:      "1080p",
			VideoDurationSeconds: 12,
			MediaType:            "video",
			Duration:             time.Second,
		},
		APIKey:  apiKey,
		User:    user,
		Account: &Account{ID: 373, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      model,
			ChannelMappedModel: mappedModel,
			BillingModelSource: BillingModelSourceChannelMapped,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, 1.20, usageRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 0.30, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, 0.30, userRepo.lastAmount, 1e-12)
	require.InDelta(t, 0.25, usageRepo.lastLog.RateMultiplier, 1e-12)
	require.NotNil(t, usageRepo.lastLog.VideoDurationSeconds)
	require.Equal(t, 12, *usageRepo.lastLog.VideoDurationSeconds)
	require.NotNil(t, usageRepo.lastLog.BillingMode)
	require.Equal(t, string(BillingModeVideo), *usageRepo.lastLog.BillingMode)
}

func TestRecordExplicitDurationSeedanceUsesRequestedChannelPriceAfterMappedTokenCandidate(t *testing.T) {
	groupID := int64(74)
	model := "peterai-seedance-channel"
	mappedModel := "seedance-upstream-token"
	inputPrice := 1e-6
	outputPrice := 2e-6
	requestPrice := 0.20
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)

	cache := newEmptyChannelCache()
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: mappedModel}] = &ChannelModelPricing{
		BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice,
	}
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: model}] = &ChannelModelPricing{
		BillingMode: BillingModePerRequest,
		Intervals:   []PricingInterval{{TierLabel: "1080p", PerRequestPrice: &requestPrice}},
	}
	cache.channelByGroupID[groupID] = &Channel{ID: 1, Status: StatusActive}
	cache.groupPlatform[groupID] = PlatformOpenAI
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	svc.channelService = channelService
	svc.resolver = NewModelPricingResolver(channelService, svc.billingService)

	group := &Group{
		ID:                   groupID,
		Platform:             PlatformOpenAI,
		RateMultiplier:       2,
		VideoRateIndependent: true,
		VideoRateMultiplier:  0.25,
		// A group token card must not shadow the direct channel media price.
		ModelPricing: []ChannelModelPricing{{
			Models: []string{model}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice,
		}},
	}
	user := &User{ID: 174}
	apiKey := &APIKey{ID: 274, UserID: user.ID, User: user, GroupID: &groupID, Group: group}
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:            "managed_seedance_requested_channel_price",
			Model:                model,
			BillingModel:         model,
			UpstreamModel:        mappedModel,
			VideoCount:           1,
			VideoResolution:      "1080p",
			VideoDurationSeconds: 12,
			MediaType:            "video",
			Duration:             time.Second,
		},
		APIKey:  apiKey,
		User:    user,
		Account: &Account{ID: 374, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      model,
			ChannelMappedModel: mappedModel,
			BillingModelSource: BillingModelSourceChannelMapped,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, 0.20, usageRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 0.05, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, 0.05, userRepo.lastAmount, 1e-12)
	require.InDelta(t, 0.25, usageRepo.lastLog.RateMultiplier, 1e-12)
	require.NotNil(t, usageRepo.lastLog.BillingMode)
	require.Equal(t, string(BillingModeVideo), *usageRepo.lastLog.BillingMode)
}

type managedMediaOwnershipCache struct {
	stubGatewayCache
	ttl time.Duration
}

func (c *managedMediaOwnershipCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	c.ttl = ttl
	return c.stubGatewayCache.SetSessionAccountID(ctx, groupID, sessionHash, accountID, ttl)
}

func TestOpenAIManagedMediaTaskOwnershipIsPrincipalAndLifecycleScoped(t *testing.T) {
	groupID := int64(7)
	cache := &managedMediaOwnershipCache{}
	svc := &OpenAIGatewayService{cache: cache, cfg: &config.Config{}}

	require.NoError(t, svc.BindOpenAIManagedMediaTaskAccount(
		context.Background(), &groupID, "task-managed", 11, 22, 33,
	))
	require.GreaterOrEqual(t, cache.ttl, 24*time.Hour)
	accountID, err := svc.ResolveOpenAIManagedMediaTaskAccount(
		context.Background(), &groupID, "task-managed", 11, 22,
	)
	require.NoError(t, err)
	require.Equal(t, int64(33), accountID)

	for _, tc := range []struct {
		name     string
		taskID   string
		userID   int64
		apiKeyID int64
	}{
		{name: "task", taskID: "task-other", userID: 11, apiKeyID: 22},
		{name: "user", taskID: "task-managed", userID: 12, apiKeyID: 22},
		{name: "api key", taskID: "task-managed", userID: 11, apiKeyID: 23},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, resolveErr := svc.ResolveOpenAIManagedMediaTaskAccount(
				context.Background(), &groupID, tc.taskID, tc.userID, tc.apiKeyID,
			)
			require.Error(t, resolveErr)
		})
	}

	// Native v3 stores the same upstream ID under a different ownership
	// namespace. A v1 lookup must not accept that binding.
	require.NoError(t, svc.BindGrokMediaVideoRequestAccount(
		context.Background(), &groupID, "task-native", 11, 22, 44,
	))
	_, err = svc.ResolveOpenAIManagedMediaTaskAccount(
		context.Background(), &groupID, "task-native", 11, 22,
	)
	require.Error(t, err)
	require.NotEqual(t,
		OpenAIManagedMediaTaskSessionHash("same-task", 11, 22),
		GrokMediaVideoRequestSessionHash("same-task", 11, 22),
	)
}

func TestForwardOpenAIManagedMediaSeedanceLookupMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		endpoint OpenAIManagedMediaEndpoint
		method   string
	}{
		{name: "status", endpoint: OpenAIManagedMediaSeedanceStatus, method: http.MethodGet},
		{name: "delete", endpoint: OpenAIManagedMediaSeedanceDelete, method: http.MethodDelete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(tc.method, "/v1/contents/generations/tasks/task-123", nil)
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"task-123","status":"running"}`)),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"api_key": "ark-test", "base_url": "https://ark.example/api/plan/v3", "openai_capabilities": []any{"seedance"},
			}}

			_, err := svc.ForwardOpenAIManagedMedia(
				context.Background(), c, account, tc.endpoint, "task-123", nil, OpenAIManagedMediaRequest{}, "",
			)

			require.NoError(t, err)
			require.Equal(t, tc.method, upstream.lastReq.Method)
			require.Equal(t, "https://ark.example/api/plan/v3/contents/generations/tasks/task-123", upstream.lastReq.URL.String())
		})
	}
}
