-- 支付渠道支持 KWD/BHD 等最多三位小数的 ISO 4217 币种。
-- pay_amount 保存网关实际收款金额，必须在入库时保留第三位小数；
-- 总精度从 20 增至 21，以维持原 DECIMAL(20,2) 的 18 位整数容量。
-- amount/bonus_amount/refund_amount 均为站内到账或退款额度，继续使用两位小数。
ALTER TABLE payment_orders
    ALTER COLUMN pay_amount TYPE DECIMAL(21,3)
    USING pay_amount::DECIMAL(21,3);
