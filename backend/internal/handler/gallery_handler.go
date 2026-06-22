package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type GalleryHandler struct {
	service *service.GalleryService
}

func NewGalleryHandler(service *service.GalleryService) *GalleryHandler {
	return &GalleryHandler{service: service}
}

func (h *GalleryHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListVisible(c.Request.Context(), page, pageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *GalleryHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	var input service.GalleryCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	items, err := h.service.Create(c.Request.Context(), subject.UserID, input)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, items)
}

func (h *GalleryHandler) ListMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListMine(c.Request.Context(), subject.UserID, page, pageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *GalleryHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteOwn(c.Request.Context(), subject.UserID, id); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *GalleryHandler) ServeMedia(c *gin.Context) {
	rel := strings.TrimPrefix(c.Param("path"), "/")
	path, ok := h.service.ResolveMediaPath(c.Request.Context(), rel)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=604800, immutable")
	c.File(path)
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid ID")
		return 0, false
	}
	return id, true
}
