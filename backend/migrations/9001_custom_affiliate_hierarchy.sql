-- 多级代理层级与差价返利审计字段。
-- 该迁移只扩展审计/授权结构，不回算历史返利。

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS affiliate_level INTEGER NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS downstream_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS rebate_base_amount DECIMAL(20,8) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS rebate_rate_percent DECIMAL(8,4) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS recipient_rate_percent DECIMAL(8,4) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS downstream_rate_percent DECIMAL(8,4) NULL;

COMMENT ON COLUMN user_affiliate_ledger.affiliate_level IS '多级返利层级：1=直接邀请人，2=上上级，NULL=非返利或历史数据';
COMMENT ON COLUMN user_affiliate_ledger.downstream_user_id IS '本级返利对应的直接下游用户；一级时通常为付款用户';
COMMENT ON COLUMN user_affiliate_ledger.rebate_base_amount IS '本次返利计算基数（支付订单金额）';
COMMENT ON COLUMN user_affiliate_ledger.rebate_rate_percent IS '本级实际差价返利比例';
COMMENT ON COLUMN user_affiliate_ledger.recipient_rate_percent IS '收款代理自身有效返利比例';
COMMENT ON COLUMN user_affiliate_ledger.downstream_rate_percent IS '直接下游代理有效返利比例；一级通常为 0';

CREATE INDEX IF NOT EXISTS idx_ual_affiliate_hierarchy_order
    ON user_affiliate_ledger (source_order_id, affiliate_level, user_id)
    WHERE action = 'accrue' AND source_order_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ual_affiliate_hierarchy_source
    ON user_affiliate_ledger (source_user_id, user_id, created_at)
    WHERE action = 'accrue' AND source_user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS affiliate_agent_access (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    notes TEXT NOT NULL DEFAULT '',
    created_by_admin_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE affiliate_agent_access IS '未来开放普通代理查看团队层级页面的授权表';
COMMENT ON COLUMN affiliate_agent_access.enabled IS '是否允许该用户访问代理团队页面';
COMMENT ON COLUMN affiliate_agent_access.notes IS '管理员备注';
COMMENT ON COLUMN affiliate_agent_access.created_by_admin_id IS '创建或最近维护该授权的管理员用户 ID';

CREATE INDEX IF NOT EXISTS idx_affiliate_agent_access_enabled
    ON affiliate_agent_access (enabled, updated_at);

CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_user
    ON user_affiliates (inviter_id, user_id);
