package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
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
	items, result, err := h.service.ListAdmin(c.Request.Context(), page, pageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *GalleryHandler) Update(c *gin.Context) {
	id, ok := parseGalleryID(c)
	if !ok {
		return
	}
	var input service.GalleryAdminUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	item, err := h.service.AdminUpdate(c.Request.Context(), id, input)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *GalleryHandler) Cleanup(c *gin.Context) {
	result, err := h.service.Cleanup(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func parseGalleryID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid ID")
		return 0, false
	}
	return id, true
}
