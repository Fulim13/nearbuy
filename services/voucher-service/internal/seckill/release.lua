-- Undo an admission. Used when the Kafka publish fails AFTER the Lua admitted:
-- the stock is already spent but no order will ever be created from it.
--
-- KEYS[1]  seckill:stock:{voucherId}
-- KEYS[2]  seckill:order:{voucherId}
-- ARGV[1]  userId
--
-- returns 1 if it released, 0 if the user was not in the set (already released,
-- or never admitted) -- idempotent either way.
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then
  redis.call('SREM', KEYS[2], ARGV[1])
  redis.call('INCRBY', KEYS[1], 1)
  return 1
end
return 0
