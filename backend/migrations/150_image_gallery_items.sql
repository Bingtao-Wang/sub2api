CREATE TABLE IF NOT EXISTS image_gallery_items (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    revised_prompt TEXT NOT NULL DEFAULT '',
    model VARCHAR(128) NOT NULL DEFAULT '',
    size VARCHAR(64) NOT NULL DEFAULT '',
    quality VARCHAR(32) NOT NULL DEFAULT '',
    format VARCHAR(16) NOT NULL DEFAULT 'png',
    mode VARCHAR(32) NOT NULL DEFAULT '',
    image_path TEXT NOT NULL,
    thumb_path TEXT NOT NULL DEFAULT '',
    image_size_bytes BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'visible',
    permanent BOOLEAN NOT NULL DEFAULT false,
    featured BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT image_gallery_items_status_check CHECK (status IN ('visible', 'hidden', 'deleted')),
    CONSTRAINT image_gallery_items_format_check CHECK (format IN ('png', 'jpeg', 'webp')),
    CONSTRAINT image_gallery_items_mode_check CHECK (mode IN ('text', 'image', ''))
);

CREATE INDEX IF NOT EXISTS idx_image_gallery_visible_created
    ON image_gallery_items (featured DESC, created_at DESC, id DESC)
    WHERE status = 'visible';

CREATE INDEX IF NOT EXISTS idx_image_gallery_user_created
    ON image_gallery_items (user_id, created_at DESC, id DESC)
    WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_image_gallery_cleanup
    ON image_gallery_items (created_at ASC, id ASC)
    WHERE permanent = false AND status <> 'deleted';
