-- Seckill admission. Runs as ONE unit on Redis's single thread, so the window
-- check, the stock check, the duplicate check and both writes cannot interleave
-- with another caller.
--
-- KEYS[1]  seckill:stock:{voucherId}
-- KEYS[2]  seckill:order:{voucherId}
-- ARGV[1]  userId
-- ARGV[2]  begin_time, unix millis
-- ARGV[3]  end_time, unix millis
--
-- returns  0 admitted
--          1 out of stock
--          2 this user already has one
--          3 not started yet
--          4 already finished
--          5 no stock key -- the voucher is unknown or was never warmed
local stockKey = KEYS[1]
local orderKey = KEYS[2]
local userId   = ARGV[1]
local beginMs  = tonumber(ARGV[2])
local endMs    = tonumber(ARGV[3])

-- Redis's own clock, not the caller's: three voucher-service replicas with a few
-- hundred ms of NTP drift must not disagree about whether the sale has opened.
local t   = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)

if now < beginMs then return 3 end
if now > endMs   then return 4 end

local stock = redis.call('GET', stockKey)
if not stock then
  -- Fail CLOSED. A missing counter means the voucher is unknown or Redis was
  -- flushed before WarmStock ran; admitting on an absent key would oversell the
  -- entire voucher.
  return 5
end
if tonumber(stock) <= 0 then return 1 end

-- One per user. SISMEMBER inside the script is what makes this safe: the same
-- check in Go would race between the read and the DECRBY.
if redis.call('SISMEMBER', orderKey, userId) == 1 then return 2 end

redis.call('DECRBY', stockKey, 1)
redis.call('SADD', orderKey, userId)
return 0
