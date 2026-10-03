package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenAIManagedMediaEndpoint 是 PeterAI 托管媒体端点。只有后台显式标记了
// 对应能力的 OpenAI APIKey 账号才会被调度，避免把音频/Seedance 请求误发给 Codex 账号。
type OpenAIManagedMediaEndpoint string

const (
	OpenAIManagedMediaAudioSpeech    OpenAIManagedMediaEndpoint = "audio_speech"
	OpenAIManagedMediaSeedanceCreate OpenAIManagedMediaEndpoint = "seedance_create"
	OpenAIManagedMediaSeedanceStatus OpenAIManagedMediaEndpoint = "seedance_status"
	OpenAIManagedMediaSeedanceDelete OpenAIManagedMediaEndpoint = "seedance_delete"
)

func (e OpenAIManagedMediaEndpoint) Capability() OpenAIEndpointCapability {
	if e == OpenAIManagedMediaAudioSpeech {
		return OpenAIEndpointCapabilityAudioSpeech
	}
	return OpenAIEndpointCapabilitySeedance
}

func (e OpenAIManagedMediaEndpoint) RequiresBody() bool {
	return e == OpenAIManagedMediaAudioSpeech || e == OpenAIManagedMediaSeedanceCreate
}

func (e OpenAIManagedMediaEndpoint) IsBillable() bool {
	return e == OpenAIManagedMediaAudioSpeech || e == OpenAIManagedMediaSeedanceCreate
}

func (e OpenAIManagedMediaEndpoint) MediaType() string {
	if e == OpenAIManagedMediaAudioSpeech {
		return "audio"
	}
	return "video"
}

func (e OpenAIManagedMediaEndpoint) upstreamPath(taskID string) string {
	switch e {
	case OpenAIManagedMediaAudioSpeech:
		return "/v1/audio/speech"
	case OpenAIManagedMediaSeedanceCreate:
		return "/v1/contents/generations/tasks"
	case OpenAIManagedMediaSeedanceStatus, OpenAIManagedMediaSeedanceDelete:
		return "/v1/contents/generations/tasks/" + strings.TrimSpace(taskID)
	default:
		return ""
	}
}

type OpenAIManagedMediaRequest struct {
	Model           string
	Resolution      string
	DurationSeconds int
	DurationAuto    bool
}

func ParseOpenAIManagedMediaRequest(endpoint OpenAIManagedMediaEndpoint, body []byte) (OpenAIManagedMediaRequest, error) {
	if !endpoint.RequiresBody() {
		return OpenAIManagedMediaRequest{}, nil
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return OpenAIManagedMediaRequest{}, errors.New("request body must be valid JSON")
	}
	if err := validateOpenAIManagedMediaJSON(body); err != nil {
		return OpenAIManagedMediaRequest{}, err
	}
	modelValue := gjson.GetBytes(body, "model")
	if !modelValue.Exists() || modelValue.Type != gjson.String {
		return OpenAIManagedMediaRequest{}, errors.New("model must be a string")
	}
	model := strings.TrimSpace(modelValue.String())
	if model == "" {
		return OpenAIManagedMediaRequest{}, errors.New("model is required")
	}
	if endpoint == OpenAIManagedMediaAudioSpeech {
		inputValue := gjson.GetBytes(body, "input")
		if !inputValue.Exists() || inputValue.Type != gjson.String || strings.TrimSpace(inputValue.String()) == "" {
			return OpenAIManagedMediaRequest{}, errors.New("input must be a non-empty string")
		}
	}
	resolution := VideoBillingResolution480P
	if endpoint == OpenAIManagedMediaSeedanceCreate {
		resolutionValue := gjson.GetBytes(body, "resolution")
		if resolutionValue.Exists() && resolutionValue.Type != gjson.Null && resolutionValue.Type != gjson.String {
			return OpenAIManagedMediaRequest{}, errors.New("resolution must be a string")
		}
		if rawResolution := strings.TrimSpace(resolutionValue.String()); rawResolution != "" {
			var ok bool
			resolution, ok = LookupVideoBillingResolution(rawResolution)
			if !ok {
				return OpenAIManagedMediaRequest{}, fmt.Errorf("resolution must be one of 480p, 720p, or 1080p")
			}
		}
	}
	duration := 0
	durationAuto := true
	if endpoint == OpenAIManagedMediaSeedanceCreate {
		durationValue := gjson.GetBytes(body, "duration")
		if durationValue.Exists() && durationValue.Type != gjson.Null {
			durationAuto = false
			switch durationValue.Type {
			case gjson.Number:
				rawDuration := durationValue.Float()
				if math.Trunc(rawDuration) != rawDuration {
					return OpenAIManagedMediaRequest{}, errors.New("duration must be an integer number of seconds")
				}
				duration = int(rawDuration)
			case gjson.String:
				var parseErr error
				duration, parseErr = strconv.Atoi(strings.TrimSpace(durationValue.String()))
				if parseErr != nil {
					return OpenAIManagedMediaRequest{}, errors.New("duration must be an integer number of seconds")
				}
			default:
				return OpenAIManagedMediaRequest{}, errors.New("duration must be an integer number of seconds")
			}
			if duration < VideoBillingMinDurationSeconds || duration > VideoBillingMaxDurationSeconds {
				return OpenAIManagedMediaRequest{}, fmt.Errorf(
					"duration must be between %d and %d seconds",
					VideoBillingMinDurationSeconds,
					VideoBillingMaxDurationSeconds,
				)
			}
		}
	}
	if durationAuto {
		duration = VideoBillingDefaultDurationSeconds
	}
	return OpenAIManagedMediaRequest{
		Model:           model,
		Resolution:      resolution,
		DurationSeconds: duration,
		DurationAuto:    durationAuto,
	}, nil
}

func validateOpenAIManagedMediaJSON(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := validateOpenAIManagedMediaJSONValue(decoder, openAIManagedMediaJSONScopeTopLevel); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return fmt.Errorf("invalid request JSON: %w", err)
	}
	return nil
}

type openAIManagedMediaJSONScope uint8

const (
	openAIManagedMediaJSONScopeOther openAIManagedMediaJSONScope = iota
	openAIManagedMediaJSONScopeTopLevel
	openAIManagedMediaJSONScopeContent
	openAIManagedMediaJSONScopeContentItem
	openAIManagedMediaJSONScopeImageURL
)

func isOpenAIManagedMediaCanonicalJSONField(scope openAIManagedMediaJSONScope, foldedKey string) bool {
	switch scope {
	case openAIManagedMediaJSONScopeTopLevel:
		switch foldedKey {
		case "model", "input", "resolution", "duration", "content":
			return true
		}
	case openAIManagedMediaJSONScopeContentItem:
		switch foldedKey {
		case "type", "text", "image_url":
			return true
		}
	case openAIManagedMediaJSONScopeImageURL:
		return foldedKey == "url"
	}
	return false
}

func openAIManagedMediaJSONFieldScope(scope openAIManagedMediaJSONScope, foldedKey string) openAIManagedMediaJSONScope {
	switch scope {
	case openAIManagedMediaJSONScopeTopLevel:
		if foldedKey == "content" {
			return openAIManagedMediaJSONScopeContent
		}
	case openAIManagedMediaJSONScopeContentItem:
		if foldedKey == "image_url" {
			return openAIManagedMediaJSONScopeImageURL
		}
	}
	return openAIManagedMediaJSONScopeOther
}

func validateOpenAIManagedMediaJSONValue(decoder *json.Decoder, scope openAIManagedMediaJSONScope) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("invalid request JSON: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return fmt.Errorf("invalid request JSON object: %w", keyErr)
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid request JSON object key")
			}
			foldedKey := strings.ToLower(key)
			if _, duplicate := seen[foldedKey]; duplicate {
				return fmt.Errorf("duplicate JSON field %q is not allowed", key)
			}
			seen[foldedKey] = struct{}{}
			if isOpenAIManagedMediaCanonicalJSONField(scope, foldedKey) && key != foldedKey {
				return fmt.Errorf("JSON field %q must use canonical lowercase spelling", key)
			}
			if err := validateOpenAIManagedMediaJSONValue(decoder, openAIManagedMediaJSONFieldScope(scope, foldedKey)); err != nil {
				return err
			}
		}
		end, endErr := decoder.Token()
		if endErr != nil || end != json.Delim('}') {
			return errors.New("invalid request JSON object")
		}
	case '[':
		itemScope := openAIManagedMediaJSONScopeOther
		if scope == openAIManagedMediaJSONScopeContent {
			itemScope = openAIManagedMediaJSONScopeContentItem
		}
		for decoder.More() {
			if err := validateOpenAIManagedMediaJSONValue(decoder, itemScope); err != nil {
				return err
			}
		}
		end, endErr := decoder.Token()
		if endErr != nil || end != json.Delim(']') {
			return errors.New("invalid request JSON array")
		}
	default:
		return errors.New("invalid request JSON delimiter")
	}
	return nil
}

func OpenAIManagedMediaModerationBody(endpoint OpenAIManagedMediaEndpoint, body []byte) []byte {
	if !gjson.ValidBytes(body) {
		return nil
	}
	payload := map[string]any{}
	switch endpoint {
	case OpenAIManagedMediaAudioSpeech:
		if input := strings.TrimSpace(gjson.GetBytes(body, "input").String()); input != "" {
			payload["prompt"] = input
		}
	case OpenAIManagedMediaSeedanceCreate:
		texts := make([]string, 0)
		images := make([]map[string]string, 0)
		for _, item := range gjson.GetBytes(body, "content").Array() {
			switch strings.ToLower(strings.TrimSpace(item.Get("type").String())) {
			case "text", "input_text":
				if text := strings.TrimSpace(item.Get("text").String()); text != "" {
					texts = append(texts, text)
				}
			case "image_url", "input_image":
				url := strings.TrimSpace(item.Get("image_url.url").String())
				if url == "" {
					url = strings.TrimSpace(item.Get("image_url").String())
				}
				if url != "" {
					images = append(images, map[string]string{"image_url": url})
				}
			}
		}
		if len(texts) > 0 {
			payload["prompt"] = strings.Join(texts, "\n")
		}
		if len(images) > 0 {
			payload["images"] = images
		}
	}
	if len(payload) == 0 {
		return nil
	}
	moderationBody, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return moderationBody
}

func hasOpenAIManagedMediaPerRequestPrice(pricing *ChannelModelPricing, tierLabel string) bool {
	if pricing == nil || (pricing.BillingMode != BillingModePerRequest && pricing.BillingMode != BillingModeImage) {
		return false
	}
	if pricing.PerRequestPrice != nil {
		return true
	}
	tierLabel = strings.TrimSpace(tierLabel)
	if tierLabel == "" {
		return false
	}
	for _, interval := range pricing.Intervals {
		if interval.PerRequestPrice != nil && strings.EqualFold(strings.TrimSpace(interval.TierLabel), tierLabel) {
			return true
		}
	}
	return false
}

const openAIManagedMediaTaskOwnershipTTL = 24 * time.Hour

// OpenAIManagedMediaTaskSessionHash scopes PeterAI's managed v1 task ownership
// to the authenticated principal. Its namespace is deliberately distinct from
// GrokMediaVideoRequestSessionHash, which also backs native Seedance v3 tasks.
// A task created through one lifecycle therefore cannot be looked up through
// the other, even when the upstream happens to return the same task ID.
func OpenAIManagedMediaTaskSessionHash(taskID string, userID, apiKeyID int64) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || userID <= 0 || apiKeyID <= 0 {
		return ""
	}
	ownerSeed := fmt.Sprintf("%d:%d:%s", userID, apiKeyID, taskID)
	return "managed-seedance:" + DeriveSessionHashFromSeed(ownerSeed)
}

// BindOpenAIManagedMediaTaskAccount durably records the account that owns a
// managed v1 Seedance task. Async jobs can outlive the normal sticky-session
// window, so the ownership TTL is at least one day.
func (s *OpenAIGatewayService) BindOpenAIManagedMediaTaskAccount(
	ctx context.Context,
	groupID *int64,
	taskID string,
	userID, apiKeyID, accountID int64,
) error {
	if s == nil || s.cache == nil {
		return errors.New("managed Seedance task ownership cache is unavailable")
	}
	cacheKey := s.openAISessionCacheKey(OpenAIManagedMediaTaskSessionHash(taskID, userID, apiKeyID))
	if cacheKey == "" || accountID <= 0 {
		return errors.New("managed Seedance task ownership is invalid")
	}
	ttl := openAIManagedMediaTaskOwnershipTTL
	if s.cfg != nil && s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		if sticky := time.Duration(s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second; sticky > ttl {
			ttl = sticky
		}
	}
	return s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), cacheKey, accountID, ttl)
}

// ResolveOpenAIManagedMediaTaskAccount returns only the account bound by a
// managed v1 create for this user and API key. Missing or foreign ownership is
// intentionally surfaced as an error so callers can fail closed with 404.
func (s *OpenAIGatewayService) ResolveOpenAIManagedMediaTaskAccount(
	ctx context.Context,
	groupID *int64,
	taskID string,
	userID, apiKeyID int64,
) (int64, error) {
	if s == nil || s.cache == nil {
		return 0, errors.New("managed Seedance task ownership cache is unavailable")
	}
	cacheKey := s.openAISessionCacheKey(OpenAIManagedMediaTaskSessionHash(taskID, userID, apiKeyID))
	if cacheKey == "" {
		return 0, errors.New("managed Seedance task ownership is invalid")
	}
	return s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), cacheKey)
}

// ValidateOpenAIManagedMediaPricing 在调用上游前强制要求站内有明确价格。
// Audio Speech 必须按次定价；Seedance 可用按次定价或分组视频每秒价。
func (s *OpenAIGatewayService) ValidateOpenAIManagedMediaPricing(ctx context.Context, apiKey *APIKey, endpoint OpenAIManagedMediaEndpoint, requestedModel, mappedModel, resolution string, durationAuto bool) error {
	if apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil {
		return errors.New("managed media requires an assigned group")
	}
	if s == nil || s.channelService == nil {
		return errors.New("managed media pricing service is unavailable")
	}
	models := usageBillingModelCandidates(mappedModel, requestedModel)
	for _, model := range models {
		pricing := s.channelService.GetChannelModelPricing(ctx, *apiKey.GroupID, model)
		tierLabel := ""
		if endpoint == OpenAIManagedMediaSeedanceCreate {
			tierLabel = NormalizeVideoBillingResolutionOrDefault(resolution)
		}
		if hasOpenAIManagedMediaPerRequestPrice(pricing, tierLabel) {
			return nil
		}
	}
	if endpoint == OpenAIManagedMediaSeedanceCreate && !durationAuto && apiKeyHasConfiguredVideoPrice(apiKey, requestedModel, resolution) {
		return nil
	}
	return fmt.Errorf("model %q has no explicit PeterAI %s price", strings.TrimSpace(requestedModel), endpoint.MediaType())
}

func (s *OpenAIGatewayService) ForwardOpenAIManagedMedia(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint OpenAIManagedMediaEndpoint,
	taskID string,
	body []byte,
	request OpenAIManagedMediaRequest,
	mappedModel string,
) (*OpenAIForwardResult, error) {
	startedAt := time.Now()
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return nil, errors.New("managed media requires an OpenAI APIKey account")
	}
	if !account.SupportsOpenAIEndpointCapability(endpoint.Capability()) {
		return nil, fmt.Errorf("account does not support %s", endpoint.Capability())
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	baseURL, err := s.validateUpstreamBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	path := endpoint.upstreamPath(taskID)
	if path == "" {
		return nil, errors.New("unsupported managed media endpoint")
	}
	targetURL := buildOpenAIEndpointURL(baseURL, path)

	upstreamBody := body
	routedModel := strings.TrimSpace(mappedModel)
	if routedModel == "" {
		routedModel = request.Model
	}
	upstreamModel := strings.TrimSpace(account.GetMappedModel(routedModel))
	if upstreamModel == "" {
		upstreamModel = routedModel
	}
	SetOpsUpstreamModel(c, upstreamModel)
	if endpoint.RequiresBody() && upstreamModel != "" {
		upstreamBody, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, fmt.Errorf("rewrite managed media model: %w", err)
		}
	}
	if endpoint == OpenAIManagedMediaSeedanceCreate {
		// Forward the exact normalized dimensions used by admission, reservation,
		// and settlement. This prevents a larger upstream request from being billed
		// at a clamped/default duration or resolution.
		upstreamBody, err = sjson.SetBytes(upstreamBody, "resolution", request.Resolution)
		if err != nil {
			return nil, fmt.Errorf("rewrite managed media resolution: %w", err)
		}
		if request.DurationAuto {
			upstreamBody, err = sjson.DeleteBytes(upstreamBody, "duration")
		} else {
			upstreamBody, err = sjson.SetBytes(upstreamBody, "duration", request.DurationSeconds)
		}
		if err != nil {
			return nil, fmt.Errorf("rewrite managed media duration: %w", err)
		}
	}

	var bodyReader io.Reader
	method := http.MethodGet
	if endpoint == OpenAIManagedMediaSeedanceDelete {
		method = http.MethodDelete
	} else if endpoint.RequiresBody() {
		method = http.MethodPost
		bodyReader = bytes.NewReader(upstreamBody)
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	req, err := http.NewRequestWithContext(upstreamCtx, method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "*/*")
	if endpoint.RequiresBody() {
		req.Header.Set("Content-Type", "application/json")
	}
	if ua := account.GetOpenAIUserAgent(); ua != "" {
		req.Header.Set("User-Agent", ua)
	} else {
		req.Header.Set("User-Agent", "sub2api-peter-media/1.0")
	}
	account.ApplyHeaderOverrides(req.Header)

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStartedAt := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStartedAt).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	requestID := firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"))
	if resp.StatusCode >= http.StatusBadRequest {
		return s.handleOpenAIManagedMediaError(ctx, c, account, resp, responseBody, requestID, upstreamModel)
	}

	responseTaskID := ""
	if endpoint == OpenAIManagedMediaSeedanceCreate {
		responseTaskID = extractOpenAIManagedMediaTaskID(responseBody)
		if responseTaskID == "" {
			setOpsUpstreamError(c, http.StatusBadGateway, "Seedance upstream did not return a task ID", "")
			return nil, errors.New("seedance upstream did not return a task ID")
		}
	}
	writeOpenAIManagedMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
	result := &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           extractOpenAIManagedMediaUsage(responseBody),
		Model:           request.Model,
		BillingModel:    routedModel,
		UpstreamModel:   upstreamModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startedAt),
		MediaType:       endpoint.MediaType(),
	}
	if endpoint == OpenAIManagedMediaAudioSpeech {
		result.RequestCount = 1
	}
	if endpoint == OpenAIManagedMediaSeedanceCreate {
		result.ResponseID = responseTaskID
		result.VideoCount = 1
		result.VideoResolution = request.Resolution
		result.VideoDurationSeconds = request.DurationSeconds
		if request.DurationAuto {
			// Auto duration is valid only with an explicit per-request price.
			// RequestCount selects that billing path and prevents the normalized
			// fallback duration from being charged as if the user requested it.
			result.RequestCount = 1
		}
	}
	return result, nil
}

func (s *OpenAIGatewayService) handleOpenAIManagedMediaError(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, body []byte, requestID, model string) (*OpenAIForwardResult, error) {
	message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if message == "" {
		message = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
	}
	setOpsUpstreamError(c, resp.StatusCode, message, "")
	if s.shouldFailoverOpenAIUpstreamResponse(account, resp.StatusCode, message, body) {
		s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, body, model)
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           body,
			RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}
	MarkResponseCommitted(c)
	writeOpenAIManagedMediaResponse(c, resp, body, s.responseHeaderFilter)
	return nil, fmt.Errorf("upstream error: %d request_id=%s", resp.StatusCode, requestID)
}

func extractOpenAIManagedMediaTaskID(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range []string{"id", "data.id", "task.id", "data.task.id"} {
		if id := strings.TrimSpace(gjson.GetBytes(body, path).String()); id != "" {
			return id
		}
	}
	return ""
}

func extractOpenAIManagedMediaUsage(body []byte) OpenAIUsage {
	usage, _ := extractOpenAIUsageFromJSONBytes(body)
	return usage
}

func writeOpenAIManagedMediaResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, filter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, body)
}
