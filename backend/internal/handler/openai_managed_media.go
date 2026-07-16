package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) AudioSpeech(c *gin.Context) {
	h.handleOpenAIManagedMedia(c, service.OpenAIManagedMediaAudioSpeech, "")
}

func (h *OpenAIGatewayHandler) SeedanceCreate(c *gin.Context) {
	h.handleOpenAIManagedMedia(c, service.OpenAIManagedMediaSeedanceCreate, "")
}

func (h *OpenAIGatewayHandler) SeedanceStatus(c *gin.Context) {
	h.handleOpenAIManagedMedia(c, service.OpenAIManagedMediaSeedanceStatus, c.Param("task_id"))
}

func (h *OpenAIGatewayHandler) handleOpenAIManagedMedia(c *gin.Context, endpoint service.OpenAIManagedMediaEndpoint, taskID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	requestStartedAt := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.managed_media",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("endpoint", string(endpoint)),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Managed media is only available for OpenAI groups")
		return
	}

	var body []byte
	var err error
	if endpoint.RequiresBody() {
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
				return
			}
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
			return
		}
	}
	request, err := service.ParseOpenAIManagedMediaRequest(endpoint, body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if endpoint == service.OpenAIManagedMediaSeedanceStatus && strings.TrimSpace(taskID) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	requestModel := request.Model
	if moderationBody := service.OpenAIManagedMediaModerationBody(endpoint, body); len(moderationBody) > 0 {
		decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, moderationBody)
		if decision != nil && decision.Blocked {
			h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
			return
		}
	}
	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, requestModel)
	if endpoint.IsBillable() {
		if err := h.gatewayService.ValidateOpenAIManagedMediaPricing(c.Request.Context(), apiKey, endpoint, requestModel, channelMapping.MappedModel, request.Resolution, request.DurationAuto); err != nil {
			reqLog.Warn("managed_media.pricing_not_configured", zap.String("model", requestModel), zap.Error(err))
			h.errorResponse(c, http.StatusServiceUnavailable, "pricing_not_configured", "PeterAI 尚未配置该媒体模型的计费价格")
			return
		}
	}

	setOpsRequestContext(c, requestModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStartedAt).Milliseconds())

	userRelease, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}
	if endpoint.IsBillable() {
		if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	if endpoint == service.OpenAIManagedMediaSeedanceStatus {
		sessionHash = service.OpenAIManagedMediaTaskSessionHash(taskID)
	}
	requestCtx := c.Request.Context()
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	switchCount := 0
	maxAccountSwitches := h.maxAccountSwitches
	if maxAccountSwitches <= 0 {
		maxAccountSwitches = 3
	}
	routingStartedAt := time.Now()

	for {
		selection, _, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			requestCtx,
			apiKey.GroupID,
			"",
			sessionHash,
			requestModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportHTTPSSE,
			endpoint.Capability(),
			false,
			false,
			false,
		)
		if err != nil || selection == nil || selection.Account == nil {
			if len(failedAccountIDs) == 0 {
				message := "No PeterAI account is configured for this media endpoint"
				h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", message)
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
			} else {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			}
			return
		}

		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		accountRelease, accountAcquired := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, false, &streamStarted, reqLog)
		if !accountAcquired {
			return
		}
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStartedAt).Milliseconds())
		writerSizeBefore := c.Writer.Size()
		result, forwardErr := func() (*service.OpenAIForwardResult, error) {
			defer func() {
				if accountRelease != nil {
					accountRelease()
				}
			}()
			return h.gatewayService.ForwardOpenAIManagedMedia(requestCtx, c, account, endpoint, taskID, body, request, channelMapping.MappedModel)
		}()
		if forwardErr != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(forwardErr, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), false, nil)
				if c.Writer.Size() != writerSizeBefore {
					h.handleFailoverExhausted(c, failoverErr, true)
					return
				}
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						select {
						case <-requestCtx.Done():
							return
						case <-time.After(sameAccountRetryDelay):
						}
						continue
					}
				}
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				switchCount++
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), false, nil)
			if c.Writer.Size() == writerSizeBefore {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			}
			reqLog.Warn("managed_media.forward_failed", zap.Int64("account_id", account.ID), zap.Error(forwardErr))
			return
		}

		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), true, nil)
		if endpoint == service.OpenAIManagedMediaSeedanceCreate && result != nil && strings.TrimSpace(result.ResponseID) != "" {
			if err := h.gatewayService.BindOpenAIManagedMediaTaskAccount(requestCtx, apiKey.GroupID, result.ResponseID, account.ID); err != nil {
				reqLog.Warn("managed_media.bind_task_account_failed", zap.String("task_id", result.ResponseID), zap.Error(err))
			}
		}
		if endpoint.IsBillable() && result != nil {
			recordOpenAIManagedMediaUsage(c, h, reqLog, apiKey, subject, subscription, account, result, requestModel, body, channelMapping.ToUsageFields(requestModel, result.UpstreamModel))
		}
		return
	}
}

func recordOpenAIManagedMediaUsage(
	c *gin.Context,
	h *OpenAIGatewayHandler,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	subscription *service.UserSubscription,
	account *service.Account,
	result *service.OpenAIForwardResult,
	requestModel string,
	body []byte,
	channelUsage service.ChannelUsageFields,
) {
	input := &service.OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    GetInboundEndpoint(c),
		UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
		UserAgent:          c.GetHeader("User-Agent"),
		IPAddress:          ip.GetClientIP(c),
		RequestPayloadHash: service.HashUsageRequestPayload(body),
		APIKeyService:      h.apiKeyService,
		QuotaPlatform:      service.QuotaPlatform(c.Request.Context(), apiKey),
		ChannelUsageFields: channelUsage,
	}
	h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, input); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.managed_media"),
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.String("model", requestModel),
				zap.Int64("account_id", account.ID),
			).Error("managed_media.record_usage_failed", zap.Error(err))
			reqLog.Debug("managed_media.record_usage_failed", zap.Error(err))
		}
	})
}
