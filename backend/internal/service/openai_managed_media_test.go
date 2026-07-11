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

	_, err = ParseOpenAIManagedMediaRequest(OpenAIManagedMediaAudioSpeech, []byte(`{"model":"gpt-4o-mini-tts"}`))
	require.ErrorContains(t, err, "input is required")
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
	}}
	parsed, err := ParseOpenAIManagedMediaRequest(OpenAIManagedMediaAudioSpeech, body)
	require.NoError(t, err)

	result, err := svc.ForwardOpenAIManagedMedia(context.Background(), c, account, OpenAIManagedMediaAudioSpeech, "", body, parsed, "tts-upstream")

	require.NoError(t, err)
	require.Equal(t, "https://speech.example/v1/audio/speech", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "tts-upstream", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "audio/mpeg", recorder.Header().Get("Content-Type"))
	require.Equal(t, "audio-bytes", recorder.Body.String())
	require.Equal(t, 1, result.RequestCount)
	require.Equal(t, "audio", result.MediaType)
	require.Equal(t, "speech-1", result.RequestID)
}

func TestForwardOpenAIManagedMediaSeedanceUsesArkVersionedBase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"seedance-public","content":[{"type":"text","text":"waves"}],"resolution":"720p","duration":6}`)
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

	result, err := svc.ForwardOpenAIManagedMedia(context.Background(), c, account, OpenAIManagedMediaSeedanceCreate, "", body, parsed, "seedance-upstream")

	require.NoError(t, err)
	require.Equal(t, "https://ark.example/api/plan/v3/contents/generations/tasks", upstream.lastReq.URL.String())
	require.Equal(t, "seedance-upstream", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "task-123", result.ResponseID)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, "720p", result.VideoResolution)
	require.Equal(t, 6, result.VideoDurationSeconds)
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
	channelService := NewChannelService(nil, nil, nil, nil)
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
	cost, err := svc.calculateOpenAIRequestCost(context.Background(), []string{"missing-model", "tts"}, apiKey, 1, 2)
	require.NoError(t, err)
	require.InDelta(t, 0.04, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.08, cost.ActualCost, 1e-9)

	price720 := 0.1
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: "seedance"}] = &ChannelModelPricing{
		BillingMode: BillingModePerRequest,
		Intervals:   []PricingInterval{{TierLabel: "720p", PerRequestPrice: &price720}},
	}
	require.NoError(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "720p", true))
	require.Error(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "1080p", false))

	price1080 := 0.2
	apiKey.Group.VideoPrice1080P = &price1080
	require.NoError(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "1080p", false))
	require.Error(t, svc.ValidateOpenAIManagedMediaPricing(context.Background(), apiKey, OpenAIManagedMediaSeedanceCreate, "seedance", "", "1080p", true))
}
