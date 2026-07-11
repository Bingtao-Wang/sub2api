package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
)

func (e OpenAIManagedMediaEndpoint) Capability() OpenAIEndpointCapability {
	if e == OpenAIManagedMediaAudioSpeech {
		return OpenAIEndpointCapabilityAudioSpeech
	}
	return OpenAIEndpointCapabilitySeedance
}

func (e OpenAIManagedMediaEndpoint) RequiresBody() bool {
	return e != OpenAIManagedMediaSeedanceStatus
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
	case OpenAIManagedMediaSeedanceStatus:
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
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		return OpenAIManagedMediaRequest{}, errors.New("model is required")
	}
	if endpoint == OpenAIManagedMediaAudioSpeech && strings.TrimSpace(gjson.GetBytes(body, "input").String()) == "" {
		return OpenAIManagedMediaRequest{}, errors.New("input is required")
	}
	duration := int(gjson.GetBytes(body, "duration").Int())
	if raw := strings.TrimSpace(gjson.GetBytes(body, "duration").String()); duration == 0 && raw != "" {
		duration, _ = strconv.Atoi(raw)
	}
	return OpenAIManagedMediaRequest{
		Model:           model,
		Resolution:      NormalizeVideoBillingResolutionOrDefault(gjson.GetBytes(body, "resolution").String()),
		DurationSeconds: NormalizeVideoBillingDurationSecondsOrDefault(duration),
		DurationAuto:    duration <= 0,
	}, nil
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

func OpenAIManagedMediaTaskSessionHash(taskID string) string {
	if taskID = strings.TrimSpace(taskID); taskID != "" {
		return "seedance-task:" + DeriveSessionHashFromSeed(taskID)
	}
	return ""
}

func (s *OpenAIGatewayService) BindOpenAIManagedMediaTaskAccount(ctx context.Context, groupID *int64, taskID string, accountID int64) error {
	return s.BindStickySession(ctx, groupID, OpenAIManagedMediaTaskSessionHash(taskID), accountID)
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
		if pricing == nil || (pricing.BillingMode != BillingModePerRequest && pricing.BillingMode != BillingModeImage) {
			continue
		}
		if pricing.PerRequestPrice != nil {
			return nil
		}
		if endpoint == OpenAIManagedMediaAudioSpeech {
			continue
		}
		for _, interval := range pricing.Intervals {
			if interval.PerRequestPrice != nil && strings.EqualFold(strings.TrimSpace(interval.TierLabel), NormalizeVideoBillingResolutionOrDefault(resolution)) {
				return nil
			}
		}
	}
	if endpoint == OpenAIManagedMediaSeedanceCreate && !durationAuto && apiKeyHasConfiguredVideoPrice(apiKey, resolution) {
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
	upstreamModel := strings.TrimSpace(mappedModel)
	if upstreamModel == "" {
		upstreamModel = request.Model
	}
	if endpoint.RequiresBody() && upstreamModel != "" && upstreamModel != request.Model {
		upstreamBody, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, fmt.Errorf("rewrite managed media model: %w", err)
		}
	}

	var bodyReader io.Reader
	method := http.MethodGet
	if endpoint.RequiresBody() {
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
		BillingModel:    request.Model,
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
	}
	return result, nil
}

func (s *OpenAIGatewayService) handleOpenAIManagedMediaError(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, body []byte, requestID, model string) (*OpenAIForwardResult, error) {
	message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if message == "" {
		message = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
	}
	setOpsUpstreamError(c, resp.StatusCode, message, "")
	if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, message, body) {
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
