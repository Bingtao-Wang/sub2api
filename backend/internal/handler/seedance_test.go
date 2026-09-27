//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedanceHandlerLifecycleAndOwnership(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	var owner int64
	upstream.call = func(req *http.Request, id int64) (*http.Response, error) {
		body := `{"id":"task-ark","status":"queued"}`
		if req.Method == http.MethodPost {
			owner = id
		} else {
			require.Equal(t, owner, id)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	newContext := func(method string) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), method == http.MethodPost)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.Group.Platform = service.PlatformOpenAI
		body := ""
		if method == http.MethodPost {
			body = `{"model":"doubao-seedance","content":[{"type":"text","text":"waves"}]}`
		}
		c.Request = httptest.NewRequest(method, "/api/v3/contents/generations/tasks", strings.NewReader(body))
		c.Params = gin.Params{{Key: "task_id", Value: "task-ark"}}
		return c, w
	}
	c, w := newContext(http.MethodPost)
	h.SeedanceTasks(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Positive(t, owner)
	require.Len(t, bindings.pending, 1)
	slots.assertReleased(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		c, w = newContext(method)
		h.SeedanceTasks(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		slots.assertReleased(t)
	}
	for _, other := range []string{"user", "key", "group", "task", "provider"} {
		c, w = newContext(http.MethodGet)
		key, _ := middleware.GetAPIKeyFromContext(c)
		switch other {
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		case "key":
			key.ID = 21
		case "group":
			group := int64(25)
			key.GroupID = &group
		case "task":
			c.Params = gin.Params{{Key: "task_id", Value: "other"}}
		case "provider":
			c.Params = gin.Params{{Key: "request_id", Value: "task-ark"}}
		}
		before := upstream.calls
		if other == "provider" {
			h.GrokVideoStatus(c)
		} else {
			h.SeedanceTasks(c)
		}
		require.Equal(t, 404, w.Code, other+": "+w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
	c, _ = newContext(http.MethodGet)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	result := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 12345}, ResponseID: "seedance:task-ark"}
	for i := range 20 {
		billed := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, result.ResponseID, result)
		if i == 0 {
			require.NotNil(t, billed)
			require.Equal(t, "doubao-seedance", billed.BillingModel)
			require.Equal(t, 12345, billed.Usage.OutputTokens)
			require.Zero(t, billed.VideoCount)
		} else {
			require.Nil(t, billed)
		}
	}
	require.Len(t, bindings.billed, 1)
}

func TestManagedSeedanceStatusAndDeleteRequireManagedOwnership(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	newContext := func(method, taskID string) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), false)
		key, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		key.Group.Platform = service.PlatformOpenAI
		c.Request = httptest.NewRequest(method, "/v1/contents/generations/tasks/"+taskID, nil)
		c.Params = gin.Params{{Key: "task_id", Value: taskID}}
		return c, w
	}

	// The fixture starts with native video ownership for "task". That must not
	// authorize either managed v1 operation, and no upstream call may be attempted.
	var c *gin.Context
	var w *httptest.ResponseRecorder
	for _, tc := range []struct {
		method string
		call   func(*gin.Context)
	}{
		{method: http.MethodGet, call: h.SeedanceStatus},
		{method: http.MethodDelete, call: h.SeedanceDelete},
	} {
		c, w = newContext(tc.method, "task")
		tc.call(c)
		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		require.Zero(t, upstream.calls)
		slots.assertReleased(t)
	}

	groupID := int64(24)
	require.NoError(t, h.gatewayService.BindOpenAIManagedMediaTaskAccount(
		context.Background(), &groupID, "managed-task", 10, 20, 1,
	))
	bindings.writes = 0
	lastMethod := ""
	upstream.call = func(req *http.Request, accountID int64) (*http.Response, error) {
		require.Equal(t, int64(1), accountID)
		lastMethod = req.Method
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"managed-task","status":"running"}`)),
		}, nil
	}

	for _, tc := range []struct {
		method string
		call   func(*gin.Context)
	}{
		{method: http.MethodGet, call: h.SeedanceStatus},
		{method: http.MethodDelete, call: h.SeedanceDelete},
	} {
		before := upstream.calls
		c, w = newContext(tc.method, "managed-task")
		tc.call(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, before+1, upstream.calls)
		require.Equal(t, tc.method, lastMethod)
		slots.assertReleased(t)
	}
	require.Zero(t, bindings.writes, "managed lookups must preserve durable ownership")

	// The inverse direction is isolated too: a managed task has no native v3
	// ownership key, so native status/delete must fail before reaching upstream.
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		c, w = newContext(method, "managed-task")
		before := upstream.calls
		h.SeedanceTasks(c)
		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}

	for _, other := range []string{"user", "api key", "task"} {
		c, w = newContext(http.MethodGet, "managed-task")
		key, _ := middleware.GetAPIKeyFromContext(c)
		switch other {
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		case "api key":
			key.ID = 21
		case "task":
			c.Params = gin.Params{{Key: "task_id", Value: "other-task"}}
		}
		before := upstream.calls
		h.SeedanceStatus(c)
		require.Equal(t, http.StatusNotFound, w.Code, other+": "+w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
}
