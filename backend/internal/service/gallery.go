package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	GalleryStatusVisible = "visible"
	GalleryStatusHidden  = "hidden"
	GalleryStatusDeleted = "deleted"

	defaultGalleryMaxImageBytes       = int64(8 * 1024 * 1024)
	defaultGalleryMaxItemsPerRequest  = 4
	defaultGalleryUserWindowLimit     = 50
	defaultGalleryRetentionDays       = 7
	defaultGalleryCleanupBatchSize    = 200
	defaultGalleryTotalMaxBytes       = int64(8 * 1024 * 1024 * 1024)
	defaultGalleryMinFreeBytes        = uint64(5 * 1024 * 1024 * 1024)
	defaultGalleryListMaxPageSize     = 50
	defaultGalleryListDefaultPageSize = 20
)

var (
	ErrGalleryItemNotFound     = infraerrors.NotFound("GALLERY_ITEM_NOT_FOUND", "gallery item not found")
	ErrGalleryForbidden        = infraerrors.Forbidden("GALLERY_FORBIDDEN", "gallery item access denied")
	ErrGalleryInvalidImage     = infraerrors.BadRequest("GALLERY_INVALID_IMAGE", "invalid gallery image")
	ErrGalleryImageTooLarge    = infraerrors.BadRequest("GALLERY_IMAGE_TOO_LARGE", "image is too large")
	ErrGalleryTooManyItems     = infraerrors.BadRequest("GALLERY_TOO_MANY_ITEMS", "too many images in one request")
	ErrGalleryUserLimitReached = infraerrors.TooManyRequests("GALLERY_USER_LIMIT_REACHED", "gallery publish limit reached")
)

type GalleryConfig struct {
	DataDir            string
	MaxImageBytes      int64
	MaxItemsPerRequest int
	UserWindowLimit    int
	RetentionDays      int
	CleanupBatchSize   int
	TotalMaxBytes      int64
	MinFreeBytes       uint64
}

type GalleryItem struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	UserName       string     `json:"user_name,omitempty"`
	Prompt         string     `json:"prompt"`
	RevisedPrompt  string     `json:"revised_prompt,omitempty"`
	Model          string     `json:"model"`
	Size           string     `json:"size"`
	Quality        string     `json:"quality"`
	Format         string     `json:"format"`
	Mode           string     `json:"mode"`
	ImagePath      string     `json:"-"`
	ThumbPath      string     `json:"-"`
	ImageURL       string     `json:"image_url"`
	ThumbURL       string     `json:"thumb_url"`
	ImageSizeBytes int64      `json:"image_size_bytes"`
	Status         string     `json:"status"`
	Permanent      bool       `json:"permanent"`
	Featured       bool       `json:"featured"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

type GalleryCreateItemInput struct {
	ImageData     string `json:"image_data"`
	ThumbData     string `json:"thumb_data"`
	Prompt        string `json:"prompt"`
	RevisedPrompt string `json:"revised_prompt"`
	Model         string `json:"model"`
	Size          string `json:"size"`
	Quality       string `json:"quality"`
	Format        string `json:"format"`
	Mode          string `json:"mode"`
}

type GalleryCreateInput struct {
	Items []GalleryCreateItemInput `json:"items"`
	// Single-item compatibility fields.
	ImageData     string `json:"image_data"`
	ThumbData     string `json:"thumb_data"`
	Prompt        string `json:"prompt"`
	RevisedPrompt string `json:"revised_prompt"`
	Model         string `json:"model"`
	Size          string `json:"size"`
	Quality       string `json:"quality"`
	Format        string `json:"format"`
	Mode          string `json:"mode"`
}

type GalleryAdminUpdateInput struct {
	Status    *string `json:"status"`
	Permanent *bool   `json:"permanent"`
	Featured  *bool   `json:"featured"`
}

type GalleryCleanupResult struct {
	DeletedRecords int   `json:"deleted_records"`
	DeletedFiles   int   `json:"deleted_files"`
	FreedBytes     int64 `json:"freed_bytes"`
}

type GalleryRepository interface {
	Create(ctx context.Context, item *GalleryItem) error
	UpdateFilePaths(ctx context.Context, id int64, imagePath, thumbPath string, imageSizeBytes int64) error
	ListVisible(ctx context.Context, params pagination.PaginationParams) ([]GalleryItem, *pagination.PaginationResult, error)
	ListAdmin(ctx context.Context, params pagination.PaginationParams) ([]GalleryItem, *pagination.PaginationResult, error)
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]GalleryItem, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*GalleryItem, error)
	SoftDelete(ctx context.Context, id int64) error
	AdminUpdate(ctx context.Context, id int64, input GalleryAdminUpdateInput) (*GalleryItem, error)
	CountUserSince(ctx context.Context, userID int64, since time.Time) (int, error)
	ListCleanupCandidates(ctx context.Context, before time.Time, limit int) ([]GalleryItem, error)
	SumStoredBytes(ctx context.Context) (int64, error)
	IsVisibleMediaPath(ctx context.Context, relPath string) (bool, error)
}

type GalleryService struct {
	repo GalleryRepository
	cfg  GalleryConfig
	now  func() time.Time

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func NewGalleryService(repo GalleryRepository, cfg GalleryConfig) *GalleryService {
	cfg = normalizeGalleryConfig(cfg)
	return &GalleryService{repo: repo, cfg: cfg, now: time.Now}
}

func normalizeGalleryConfig(cfg GalleryConfig) GalleryConfig {
	if strings.TrimSpace(cfg.DataDir) == "" {
		cfg.DataDir = "./data"
	}
	if cfg.MaxImageBytes <= 0 {
		cfg.MaxImageBytes = defaultGalleryMaxImageBytes
	}
	if cfg.MaxItemsPerRequest <= 0 || cfg.MaxItemsPerRequest > defaultGalleryMaxItemsPerRequest {
		cfg.MaxItemsPerRequest = defaultGalleryMaxItemsPerRequest
	}
	if cfg.UserWindowLimit <= 0 {
		cfg.UserWindowLimit = defaultGalleryUserWindowLimit
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = defaultGalleryRetentionDays
	}
	if cfg.CleanupBatchSize <= 0 {
		cfg.CleanupBatchSize = defaultGalleryCleanupBatchSize
	}
	if cfg.TotalMaxBytes <= 0 {
		cfg.TotalMaxBytes = defaultGalleryTotalMaxBytes
	}
	if cfg.MinFreeBytes == 0 {
		cfg.MinFreeBytes = defaultGalleryMinFreeBytes
	}
	return cfg
}

func (s *GalleryService) Create(ctx context.Context, userID int64, input GalleryCreateInput) ([]GalleryItem, error) {
	items := input.Items
	if len(items) == 0 && strings.TrimSpace(input.ImageData) != "" {
		items = []GalleryCreateItemInput{{
			ImageData:     input.ImageData,
			ThumbData:     input.ThumbData,
			Prompt:        input.Prompt,
			RevisedPrompt: input.RevisedPrompt,
			Model:         input.Model,
			Size:          input.Size,
			Quality:       input.Quality,
			Format:        input.Format,
			Mode:          input.Mode,
		}}
	}
	if len(items) == 0 {
		return nil, ErrGalleryInvalidImage
	}
	if len(items) > s.cfg.MaxItemsPerRequest {
		return nil, ErrGalleryTooManyItems
	}
	count, err := s.repo.CountUserSince(ctx, userID, s.now().AddDate(0, 0, -s.cfg.RetentionDays))
	if err != nil {
		return nil, fmt.Errorf("count gallery user window: %w", err)
	}
	if count+len(items) > s.cfg.UserWindowLimit {
		return nil, ErrGalleryUserLimitReached
	}

	created := make([]GalleryItem, 0, len(items))
	for _, raw := range items {
		item, imageBytes, thumbBytes, err := s.prepareCreateItem(userID, raw)
		if err != nil {
			s.rollbackCreated(context.Background(), created)
			return nil, err
		}
		if err := s.repo.Create(ctx, &item); err != nil {
			s.rollbackCreated(context.Background(), created)
			return nil, err
		}
		updatedItem, err := s.writeItemFiles(ctx, item, imageBytes, thumbBytes)
		if err != nil {
			_ = s.repo.SoftDelete(context.Background(), item.ID)
			s.rollbackCreated(context.Background(), created)
			return nil, err
		}
		updatedItem.ImageURL = GalleryMediaURL(updatedItem.ImagePath)
		updatedItem.ThumbURL = GalleryMediaURL(firstNonEmpty(updatedItem.ThumbPath, updatedItem.ImagePath))
		created = append(created, updatedItem)
	}
	return created, nil
}

func (s *GalleryService) rollbackCreated(ctx context.Context, items []GalleryItem) {
	for _, item := range items {
		s.removeFiles(item)
		_ = s.repo.SoftDelete(ctx, item.ID)
	}
}

func (s *GalleryService) prepareCreateItem(userID int64, raw GalleryCreateItemInput) (GalleryItem, []byte, []byte, error) {
	prompt := strings.TrimSpace(raw.Prompt)
	if prompt == "" {
		return GalleryItem{}, nil, nil, infraerrors.BadRequest("GALLERY_PROMPT_REQUIRED", "prompt is required")
	}
	imageBytes, imageExt, err := decodeGalleryImage(raw.ImageData, s.cfg.MaxImageBytes)
	if err != nil {
		return GalleryItem{}, nil, nil, err
	}
	thumbBytes := []byte(nil)
	thumbExt := "jpg"
	if strings.TrimSpace(raw.ThumbData) != "" {
		thumbBytes, thumbExt, err = decodeGalleryImage(raw.ThumbData, minInt64(s.cfg.MaxImageBytes, 1024*1024))
		if err != nil {
			return GalleryItem{}, nil, nil, err
		}
	}
	format := imageExt
	mode := normalizeGalleryMode(raw.Mode)
	datePath := s.now().Format("2006/01/02")
	item := GalleryItem{
		UserID:         userID,
		Prompt:         trimMax(prompt, 4000),
		RevisedPrompt:  trimMax(strings.TrimSpace(raw.RevisedPrompt), 4000),
		Model:          trimMax(strings.TrimSpace(raw.Model), 128),
		Size:           trimMax(strings.TrimSpace(raw.Size), 64),
		Quality:        trimMax(strings.TrimSpace(raw.Quality), 32),
		Format:         format,
		Mode:           mode,
		ImagePath:      filepath.ToSlash(filepath.Join(datePath, "pending."+format)),
		ThumbPath:      filepath.ToSlash(filepath.Join("thumbs", datePath, "pending."+thumbExt)),
		ImageSizeBytes: int64(len(imageBytes)),
		Status:         GalleryStatusHidden,
		CreatedAt:      s.now(),
		UpdatedAt:      s.now(),
	}
	if len(thumbBytes) == 0 {
		item.ThumbPath = ""
	}
	return item, imageBytes, thumbBytes, nil
}

func (s *GalleryService) writeItemFiles(ctx context.Context, item GalleryItem, imageBytes, thumbBytes []byte) (GalleryItem, error) {
	imageExt := normalizeGalleryFormat(item.Format)
	if imageExt == "jpg" {
		imageExt = "jpeg"
	}
	datePath := item.CreatedAt.Format("2006/01/02")
	imageRel := filepath.ToSlash(filepath.Join(datePath, fmt.Sprintf("%d.%s", item.ID, imageExt)))
	imagePath, err := s.resolveStoragePath(imageRel)
	if err != nil {
		return item, err
	}
	if err := os.MkdirAll(filepath.Dir(imagePath), 0755); err != nil {
		return item, fmt.Errorf("create gallery dir: %w", err)
	}
	if err := os.WriteFile(imagePath, imageBytes, 0644); err != nil {
		return item, fmt.Errorf("write gallery image: %w", err)
	}

	thumbRel := ""
	if len(thumbBytes) > 0 {
		thumbExt := "jpg"
		if item.ThumbPath != "" {
			thumbExt = strings.TrimPrefix(filepath.Ext(item.ThumbPath), ".")
		}
		thumbRel = filepath.ToSlash(filepath.Join("thumbs", datePath, fmt.Sprintf("%d.%s", item.ID, thumbExt)))
		thumbPath, err := s.resolveStoragePath(thumbRel)
		if err != nil {
			return item, err
		}
		if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
			_ = os.Remove(imagePath)
			return item, fmt.Errorf("create gallery thumb dir: %w", err)
		}
		if err := os.WriteFile(thumbPath, thumbBytes, 0644); err != nil {
			_ = os.Remove(imagePath)
			return item, fmt.Errorf("write gallery thumb: %w", err)
		}
	}
	if err := s.repo.UpdateFilePaths(ctx, item.ID, imageRel, thumbRel, int64(len(imageBytes))); err != nil {
		_, _ = s.removeFiles(GalleryItem{ImagePath: imageRel, ThumbPath: thumbRel})
		return item, err
	}
	item.ImagePath = imageRel
	item.ThumbPath = thumbRel
	item.ImageSizeBytes = int64(len(imageBytes))
	item.Status = GalleryStatusVisible
	return item, nil
}

func (s *GalleryService) StartCleanupWorker() {
	if s.stopCh != nil {
		return
	}
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	go func() {
		defer close(s.doneCh)
		for {
			timer := time.NewTimer(time.Until(nextGalleryCleanupTime(s.now())))
			select {
			case <-timer.C:
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				_, _ = s.Cleanup(ctx)
				cancel()
			case <-s.stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
		}
	}()
}

func (s *GalleryService) Stop() {
	s.stopOnce.Do(func() {
		if s.stopCh == nil {
			return
		}
		close(s.stopCh)
		<-s.doneCh
	})
}

func (s *GalleryService) ListVisible(ctx context.Context, page, pageSize int) ([]GalleryItem, *pagination.PaginationResult, error) {
	params := galleryPagination(page, pageSize)
	items, result, err := s.repo.ListVisible(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	s.attachURLs(items)
	return items, result, nil
}

func (s *GalleryService) ListAdmin(ctx context.Context, page, pageSize int) ([]GalleryItem, *pagination.PaginationResult, error) {
	params := galleryPagination(page, pageSize)
	items, result, err := s.repo.ListAdmin(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	s.attachURLs(items)
	return items, result, nil
}

func (s *GalleryService) ListMine(ctx context.Context, userID int64, page, pageSize int) ([]GalleryItem, *pagination.PaginationResult, error) {
	params := galleryPagination(page, pageSize)
	items, result, err := s.repo.ListByUser(ctx, userID, params)
	if err != nil {
		return nil, nil, err
	}
	s.attachURLs(items)
	return items, result, nil
}

func (s *GalleryService) DeleteOwn(ctx context.Context, userID, id int64) error {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item.UserID != userID {
		return ErrGalleryForbidden
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	s.removeFiles(*item)
	return nil
}

func (s *GalleryService) AdminUpdate(ctx context.Context, id int64, input GalleryAdminUpdateInput) (*GalleryItem, error) {
	if input.Status != nil {
		status := normalizeGalleryStatus(*input.Status)
		if status == "" {
			return nil, infraerrors.BadRequest("GALLERY_INVALID_STATUS", "invalid gallery status")
		}
		input.Status = &status
	}
	item, err := s.repo.AdminUpdate(ctx, id, input)
	if err != nil {
		return nil, err
	}
	item.ImageURL = GalleryMediaURL(item.ImagePath)
	item.ThumbURL = GalleryMediaURL(firstNonEmpty(item.ThumbPath, item.ImagePath))
	if input.Status != nil && *input.Status == GalleryStatusDeleted {
		s.removeFiles(*item)
	}
	return item, nil
}

func (s *GalleryService) Cleanup(ctx context.Context) (*GalleryCleanupResult, error) {
	before := s.now().AddDate(0, 0, -s.cfg.RetentionDays)
	candidates, err := s.repo.ListCleanupCandidates(ctx, before, s.cfg.CleanupBatchSize)
	if err != nil {
		return nil, err
	}
	result := &GalleryCleanupResult{}
	for _, item := range candidates {
		deletedFiles, freedBytes := s.removeFiles(item)
		result.DeletedFiles += deletedFiles
		result.FreedBytes += freedBytes
		if err := s.repo.SoftDelete(ctx, item.ID); err != nil && !errors.Is(err, ErrGalleryItemNotFound) {
			return result, err
		}
		result.DeletedRecords++
	}
	lowFree := false
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Clean(s.cfg.DataDir), &stat); err == nil {
		lowFree = stat.Bavail*uint64(stat.Bsize) < s.cfg.MinFreeBytes
	}
	if total, err := s.repo.SumStoredBytes(ctx); err == nil && (total > s.cfg.TotalMaxBytes || lowFree) {
		extra, err := s.repo.ListCleanupCandidates(ctx, s.now(), s.cfg.CleanupBatchSize)
		if err != nil {
			return result, err
		}
		for _, item := range extra {
			if item.Permanent || item.Status == GalleryStatusDeleted {
				continue
			}
			deletedFiles, freedBytes := s.removeFiles(item)
			result.DeletedFiles += deletedFiles
			result.FreedBytes += freedBytes
			if err := s.repo.SoftDelete(ctx, item.ID); err != nil && !errors.Is(err, ErrGalleryItemNotFound) {
				return result, err
			}
			result.DeletedRecords++
			total -= item.ImageSizeBytes
			if lowFree {
				if !s.isLowFreeDisk() {
					break
				}
				continue
			}
			if total <= s.cfg.TotalMaxBytes {
				break
			}
		}
	}
	return result, nil
}

func (s *GalleryService) isLowFreeDisk() bool {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Clean(s.cfg.DataDir), &stat); err != nil {
		return false
	}
	return stat.Bavail*uint64(stat.Bsize) < s.cfg.MinFreeBytes
}

func (s *GalleryService) ResolveMediaPath(ctx context.Context, relPath string) (string, bool) {
	cleaned, ok := cleanGalleryRelativePath(relPath)
	if !ok {
		return "", false
	}
	visible, err := s.repo.IsVisibleMediaPath(ctx, filepath.ToSlash(cleaned))
	if err != nil || !visible {
		return "", false
	}
	full, err := s.resolveStoragePath(cleaned)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", false
	}
	return full, true
}

func (s *GalleryService) resolveStoragePath(relPath string) (string, error) {
	cleaned, ok := cleanGalleryRelativePath(relPath)
	if !ok {
		return "", fmt.Errorf("invalid gallery path")
	}
	base := filepath.Clean(filepath.Join(s.cfg.DataDir, "gallery"))
	target := filepath.Clean(filepath.Join(base, cleaned))
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("invalid gallery path")
	}
	return target, nil
}

func (s *GalleryService) removeFiles(item GalleryItem) (int, int64) {
	deleted := 0
	var freed int64
	for _, rel := range []string{item.ImagePath, item.ThumbPath} {
		if rel == "" {
			continue
		}
		path, err := s.resolveStoragePath(rel)
		if err != nil {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			freed += info.Size()
			if os.Remove(path) == nil {
				deleted++
			}
		}
	}
	return deleted, freed
}

func (s *GalleryService) attachURLs(items []GalleryItem) {
	for i := range items {
		items[i].ImageURL = GalleryMediaURL(items[i].ImagePath)
		items[i].ThumbURL = GalleryMediaURL(firstNonEmpty(items[i].ThumbPath, items[i].ImagePath))
	}
}

func GalleryMediaURL(relPath string) string {
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	if relPath == "" {
		return ""
	}
	return "/api/v1/gallery/media/" + relPath
}

func galleryPagination(page, pageSize int) pagination.PaginationParams {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultGalleryListDefaultPageSize
	}
	if pageSize > defaultGalleryListMaxPageSize {
		pageSize = defaultGalleryListMaxPageSize
	}
	return pagination.PaginationParams{Page: page, PageSize: pageSize}
}

func decodeGalleryImage(raw string, maxBytes int64) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", ErrGalleryInvalidImage
	}
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		if comma < 0 {
			return nil, "", ErrGalleryInvalidImage
		}
		raw = raw[comma+1:]
	}
	if maxBytes > 0 && int64(base64.StdEncoding.DecodedLen(len(raw))) > maxBytes+3 {
		return nil, "", ErrGalleryImageTooLarge
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", ErrGalleryInvalidImage
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, "", ErrGalleryImageTooLarge
	}
	ext := detectGalleryImageExt(data)
	if ext == "" {
		return nil, "", ErrGalleryInvalidImage
	}
	return data, ext, nil
}

func detectGalleryImageExt(data []byte) string {
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "jpeg"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	return ""
}

func normalizeGalleryFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "jpg":
		return "jpeg"
	case "jpeg", "png", "webp":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeGalleryMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image_to_image", "image":
		return "image"
	case "text_to_image", "text", "":
		return "text"
	default:
		return "text"
	}
}

func normalizeGalleryStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case GalleryStatusVisible, GalleryStatusHidden, GalleryStatusDeleted:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func cleanGalleryRelativePath(value string) (string, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "/")
	value = strings.ReplaceAll(value, "\\", "/")
	if value == "" || strings.ContainsRune(value, 0) {
		return "", false
	}
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", false
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) == 0 {
		return "", false
	}
	return filepath.Join(cleaned...), true
}

func trimMax(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func nextGalleryCleanupTime(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 30, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
