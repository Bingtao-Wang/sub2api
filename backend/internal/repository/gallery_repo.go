package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type galleryRepository struct {
	db *sql.DB
}

func NewGalleryRepository(db *sql.DB) service.GalleryRepository {
	return &galleryRepository{db: db}
}

func (r *galleryRepository) Create(ctx context.Context, item *service.GalleryItem) error {
	if item.Status == "" {
		item.Status = service.GalleryStatusVisible
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO image_gallery_items (
			user_id, prompt, revised_prompt, model, size, quality, format, mode,
			image_path, thumb_path, image_size_bytes, status, permanent, featured
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at`,
		item.UserID, item.Prompt, item.RevisedPrompt, item.Model, item.Size, item.Quality, item.Format, item.Mode,
		item.ImagePath, item.ThumbPath, item.ImageSizeBytes, item.Status, item.Permanent, item.Featured,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create gallery item: %w", err)
	}
	return nil
}

func (r *galleryRepository) UpdateFilePaths(ctx context.Context, id int64, imagePath, thumbPath string, imageSizeBytes int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE image_gallery_items
		SET image_path = $2, thumb_path = $3, image_size_bytes = $4, status = 'visible', updated_at = NOW()
		WHERE id = $1 AND status <> 'deleted'`,
		id, imagePath, thumbPath, imageSizeBytes,
	)
	if err != nil {
		return fmt.Errorf("update gallery file paths: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return service.ErrGalleryItemNotFound
	}
	return nil
}

func (r *galleryRepository) ListVisible(ctx context.Context, params pagination.PaginationParams) ([]service.GalleryItem, *pagination.PaginationResult, error) {
	return r.list(ctx, params, "i.status = 'visible'", nil)
}

func (r *galleryRepository) ListAdmin(ctx context.Context, params pagination.PaginationParams) ([]service.GalleryItem, *pagination.PaginationResult, error) {
	return r.list(ctx, params, "i.status <> 'deleted'", nil)
}

func (r *galleryRepository) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.GalleryItem, *pagination.PaginationResult, error) {
	return r.list(ctx, params, "i.user_id = $1 AND i.status <> 'deleted'", []any{userID})
}

func (r *galleryRepository) list(ctx context.Context, params pagination.PaginationParams, where string, args []any) ([]service.GalleryItem, *pagination.PaginationResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	pageSize := params.Limit()
	var total int64
	countQuery := "SELECT COUNT(*) FROM image_gallery_items i WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count gallery items: %w", err)
	}
	queryArgs := append([]any{}, args...)
	limitPos := len(queryArgs) + 1
	offsetPos := len(queryArgs) + 2
	queryArgs = append(queryArgs, pageSize, params.Offset())
	query := fmt.Sprintf(`
		SELECT i.id, i.user_id, COALESCE(NULLIF(u.username, ''), u.email, ''), i.prompt, i.revised_prompt, i.model, i.size, i.quality, i.format, i.mode,
		       i.image_path, i.thumb_path, i.image_size_bytes, i.status, i.permanent, i.featured,
		       i.created_at, i.updated_at, i.deleted_at
		FROM image_gallery_items i
		LEFT JOIN users u ON u.id = i.user_id
		WHERE %s
		ORDER BY i.featured DESC, i.created_at DESC, i.id DESC
		LIMIT $%d OFFSET $%d`, where, limitPos, offsetPos)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("list gallery items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items, err := scanGalleryRows(rows)
	if err != nil {
		return nil, nil, err
	}
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: pageSize, Pages: pages}, nil
}

func (r *galleryRepository) GetByID(ctx context.Context, id int64) (*service.GalleryItem, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT i.id, i.user_id, COALESCE(NULLIF(u.username, ''), u.email, ''), i.prompt, i.revised_prompt, i.model, i.size, i.quality, i.format, i.mode,
		       i.image_path, i.thumb_path, i.image_size_bytes, i.status, i.permanent, i.featured,
		       i.created_at, i.updated_at, i.deleted_at
		FROM image_gallery_items i
		LEFT JOIN users u ON u.id = i.user_id
		WHERE i.id = $1`, id)
	item, err := scanGalleryRow(row)
	if err == sql.ErrNoRows {
		return nil, service.ErrGalleryItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gallery item: %w", err)
	}
	return item, nil
}

func (r *galleryRepository) SoftDelete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE image_gallery_items
		SET status = 'deleted', deleted_at = COALESCE(deleted_at, NOW()), updated_at = NOW()
		WHERE id = $1 AND status <> 'deleted'`, id)
	if err != nil {
		return fmt.Errorf("delete gallery item: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return service.ErrGalleryItemNotFound
	}
	return nil
}

func (r *galleryRepository) AdminUpdate(ctx context.Context, id int64, input service.GalleryAdminUpdateInput) (*service.GalleryItem, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{id}
	if input.Status != nil {
		args = append(args, *input.Status)
		sets = append(sets, fmt.Sprintf("status = $%d", len(args)))
		if *input.Status == service.GalleryStatusDeleted {
			sets = append(sets, "deleted_at = COALESCE(deleted_at, NOW())")
		} else {
			sets = append(sets, "deleted_at = NULL")
		}
	}
	if input.Permanent != nil {
		args = append(args, *input.Permanent)
		sets = append(sets, fmt.Sprintf("permanent = $%d", len(args)))
	}
	if input.Featured != nil {
		args = append(args, *input.Featured)
		sets = append(sets, fmt.Sprintf("featured = $%d", len(args)))
	}
	query := fmt.Sprintf("UPDATE image_gallery_items SET %s WHERE id = $1 AND status <> 'deleted'", strings.Join(sets, ", "))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("admin update gallery item: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, service.ErrGalleryItemNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *galleryRepository) CountUserSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM image_gallery_items
		WHERE user_id = $1 AND created_at >= $2 AND status <> 'deleted'`, userID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count gallery user items: %w", err)
	}
	return count, nil
}

func (r *galleryRepository) ListCleanupCandidates(ctx context.Context, before time.Time, limit int) ([]service.GalleryItem, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.user_id, COALESCE(NULLIF(u.username, ''), u.email, ''), i.prompt, i.revised_prompt, i.model, i.size, i.quality, i.format, i.mode,
		       i.image_path, i.thumb_path, i.image_size_bytes, i.status, i.permanent, i.featured,
		       i.created_at, i.updated_at, i.deleted_at
		FROM image_gallery_items i
		LEFT JOIN users u ON u.id = i.user_id
		WHERE i.permanent = false AND i.status <> 'deleted' AND i.created_at < $1
		ORDER BY i.created_at ASC, i.id ASC
		LIMIT $2`, before, limit)
	if err != nil {
		return nil, fmt.Errorf("list gallery cleanup candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanGalleryRows(rows)
}

func (r *galleryRepository) SumStoredBytes(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT SUM(image_size_bytes) FROM image_gallery_items WHERE status <> 'deleted'`).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum gallery bytes: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func (r *galleryRepository) IsVisibleMediaPath(ctx context.Context, relPath string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM image_gallery_items
			WHERE status = 'visible' AND (image_path = $1 OR thumb_path = $1)
		)`, relPath).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check gallery media visibility: %w", err)
	}
	return exists, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanGalleryRow(row rowScanner) (*service.GalleryItem, error) {
	var item service.GalleryItem
	var deletedAt sql.NullTime
	if err := row.Scan(
		&item.ID, &item.UserID, &item.UserName, &item.Prompt, &item.RevisedPrompt, &item.Model, &item.Size, &item.Quality, &item.Format, &item.Mode,
		&item.ImagePath, &item.ThumbPath, &item.ImageSizeBytes, &item.Status, &item.Permanent, &item.Featured,
		&item.CreatedAt, &item.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func scanGalleryRows(rows *sql.Rows) ([]service.GalleryItem, error) {
	items := make([]service.GalleryItem, 0)
	for rows.Next() {
		item, err := scanGalleryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan gallery item: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gallery items: %w", err)
	}
	return items, nil
}
