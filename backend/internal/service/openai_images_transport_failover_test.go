//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardOpenAIImages_ConnectErrorTriggersFailoverBeforeResponse(t *testing.T) {
	tests := []struct {
		name       string
		account    *Account
		wantTarget string
	}{
		{
			name:       "API key images passthrough",
			account:    newOpenAIImagesAPIKeyAccount(),
			wantTarget: "/v1/images/generations",
		},
		{
			name:       "OAuth Codex images",
			account:    directImagesTestAccount(),
			wantTarget: "/backend-api/codex/images/generations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"b64_json"}`)
			c, recorder := newOpenAIImagesTestContext(t, body)
			transportErr := errors.New(`Post "https://images.example.test/v1/images/generations?access_token=transport-secret": dial tcp: connection reset by peer`)
			upstream := &httpUpstreamRecorder{err: transportErr}
			svc := newOpenAIImagesTestService(upstream)
			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)

			result, err := svc.ForwardImages(context.Background(), c, tt.account, body, parsed, "")

			require.Nil(t, result)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, tt.wantTarget, upstream.lastReq.URL.Path)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
			require.JSONEq(t, `{"error":{"type":"upstream_error","message":"Upstream request failed"}}`, string(failoverErr.ResponseBody))
			require.NotContains(t, err.Error(), "transport-secret")
			require.NotContains(t, string(failoverErr.ResponseBody), "transport-secret")

			// The service must leave the response untouched so the handler can retry
			// the request with another account.
			require.False(t, c.Writer.Written())
			require.Empty(t, recorder.Body.String())

			rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.Len(t, events, 1)
			require.Equal(t, "request_error", events[0].Kind)
			require.Equal(t, tt.account.ID, events[0].AccountID)
			require.Contains(t, events[0].Message, "access_token=***")
			require.NotContains(t, events[0].Message, "transport-secret")
		})
	}
}
