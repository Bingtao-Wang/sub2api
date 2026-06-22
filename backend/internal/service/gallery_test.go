package service

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type galleryRepoStub struct {
	items  map[int64]GalleryItem
	nextID int64
	count  int
}

func newGalleryRepoStub() *galleryRepoStub {
	return &galleryRepoStub{items: make(map[int64]GalleryItem), nextID: 1}
}

func (r *galleryRepoStub) Create(_ context.Context, item *GalleryItem) error {
	item.ID = r.nextID
	r.nextID++
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	item.UpdatedAt = item.CreatedAt
	r.items[item.ID] = *item
	return nil
}

func (r *galleryRepoStub) UpdateFilePaths(_ context.Context, id int64, imagePath, thumbPath string, imageSizeBytes int64) error {
	item, ok := r.items[id]
	if !ok {
		return ErrGalleryItemNotFound
	}
	item.ImagePath = imagePath
	item.ThumbPath = thumbPath
	item.ImageSizeBytes = imageSizeBytes
	item.Status = GalleryStatusVisible
	r.items[id] = item
	return nil
}

func (r *galleryRepoStub) ListVisible(_ context.Context, params pagination.PaginationParams) ([]GalleryItem, *pagination.PaginationResult, error) {
	items := make([]GalleryItem, 0)
	for _, item := range r.items {
		if item.Status == GalleryStatusVisible {
			items = append(items, item)
		}
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *galleryRepoStub) ListAdmin(_ context.Context, params pagination.PaginationParams) ([]GalleryItem, *pagination.PaginationResult, error) {
	items := make([]GalleryItem, 0)
	for _, item := range r.items {
		if item.Status != GalleryStatusDeleted {
			items = append(items, item)
		}
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *galleryRepoStub) ListByUser(_ context.Context, _ int64, params pagination.PaginationParams) ([]GalleryItem, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *galleryRepoStub) GetByID(_ context.Context, id int64) (*GalleryItem, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, ErrGalleryItemNotFound
	}
	return &item, nil
}

func (r *galleryRepoStub) SoftDelete(_ context.Context, id int64) error {
	item, ok := r.items[id]
	if !ok {
		return ErrGalleryItemNotFound
	}
	item.Status = GalleryStatusDeleted
	now := time.Now()
	item.DeletedAt = &now
	r.items[id] = item
	return nil
}

func (r *galleryRepoStub) AdminUpdate(_ context.Context, id int64, input GalleryAdminUpdateInput) (*GalleryItem, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, ErrGalleryItemNotFound
	}
	if input.Status != nil {
		item.Status = *input.Status
	}
	if input.Permanent != nil {
		item.Permanent = *input.Permanent
	}
	if input.Featured != nil {
		item.Featured = *input.Featured
	}
	r.items[id] = item
	return &item, nil
}

func (r *galleryRepoStub) CountUserSince(_ context.Context, _ int64, _ time.Time) (int, error) {
	return r.count, nil
}

func (r *galleryRepoStub) ListCleanupCandidates(_ context.Context, before time.Time, limit int) ([]GalleryItem, error) {
	items := make([]GalleryItem, 0)
	for _, item := range r.items {
		if item.Permanent || item.Status == GalleryStatusDeleted || !item.CreatedAt.Before(before) {
			continue
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *galleryRepoStub) SumStoredBytes(_ context.Context) (int64, error) {
	return 0, nil
}

func (r *galleryRepoStub) IsVisibleMediaPath(_ context.Context, relPath string) (bool, error) {
	for _, item := range r.items {
		if item.Status == GalleryStatusVisible && (item.ImagePath == relPath || item.ThumbPath == relPath) {
			return true, nil
		}
	}
	return false, nil
}

func TestGalleryServiceCreateAcceptsPNGAndWritesFiles(t *testing.T) {
	dir := t.TempDir()
	repo := newGalleryRepoStub()
	svc := NewGalleryService(repo, GalleryConfig{DataDir: dir})
	png := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nrest"))

	items, err := svc.Create(context.Background(), 7, GalleryCreateInput{
		ImageData: png,
		Prompt:    "hello",
		Model:     "gpt-image-2",
		Format:    "png",
		Mode:      "text",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(items) != 1 || items[0].ID == 0 {
		t.Fatalf("Create() items = %#v", items)
	}
	if _, ok := svc.ResolveMediaPath(context.Background(), items[0].ImagePath); !ok {
		t.Fatalf("ResolveMediaPath(%q) failed", items[0].ImagePath)
	}
}

func TestGalleryServiceRejectsInvalidImage(t *testing.T) {
	svc := NewGalleryService(newGalleryRepoStub(), GalleryConfig{DataDir: t.TempDir()})
	_, err := svc.Create(context.Background(), 1, GalleryCreateInput{ImageData: base64.StdEncoding.EncodeToString([]byte("not image")), Prompt: "x"})
	if err == nil {
		t.Fatal("expected invalid image error")
	}
}

func TestGalleryServiceRejectsOversizeImage(t *testing.T) {
	svc := NewGalleryService(newGalleryRepoStub(), GalleryConfig{DataDir: t.TempDir(), MaxImageBytes: 12})
	data := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	_, err := svc.Create(context.Background(), 1, GalleryCreateInput{
		ImageData: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
		Prompt:    "x",
	})
	if err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestGalleryResolveMediaPathRejectsTraversal(t *testing.T) {
	svc := NewGalleryService(newGalleryRepoStub(), GalleryConfig{DataDir: t.TempDir()})
	if _, ok := svc.ResolveMediaPath(context.Background(), "../secret.png"); ok {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, ok := svc.ResolveMediaPath(context.Background(), filepath.Join("..", "secret.png")); ok {
		t.Fatal("expected cleaned traversal path to be rejected")
	}
}

func TestGalleryCleanupKeepsPermanentItems(t *testing.T) {
	dir := t.TempDir()
	repo := newGalleryRepoStub()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -8)
	repo.items[1] = GalleryItem{ID: 1, ImagePath: "2026/06/01/1.png", Status: GalleryStatusVisible, CreatedAt: old}
	repo.items[2] = GalleryItem{ID: 2, ImagePath: "2026/06/01/2.png", Status: GalleryStatusVisible, Permanent: true, CreatedAt: old}
	svc := NewGalleryService(repo, GalleryConfig{DataDir: dir, RetentionDays: 7})
	svc.now = func() time.Time { return now }

	result, err := svc.Cleanup(context.Background())
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if result.DeletedRecords != 1 {
		t.Fatalf("DeletedRecords = %d, want 1", result.DeletedRecords)
	}
	if repo.items[2].Status == GalleryStatusDeleted {
		t.Fatal("permanent item was deleted")
	}
}
