local capacity = tonumber(ARGV[1])   -- max tokens the bucket can hold
local rate = tonumber(ARGV[2])       -- tokens added back per second
local want = tonumber(ARGV[3])       -- tokens this request needs

local t = redis.call('TIME')                        -- get Redis clock {seconds, microseconds}
local now = t[1] * 1000 + math.floor(t[2] / 1000)   -- convert to milliseconds

local b = redis.call('HMGET', KEYS[1], 'tokens', 'ts')   -- load saved bucket state
local tokens = tonumber(b[1])                            -- tokens left last time
local ts = tonumber(b[2])                                -- when it was last updated
if tokens == nil then tokens = capacity; ts = now end    -- new bucket starts full

local elapsed = now - ts                                  -- ms since last update
if elapsed < 0 then elapsed = 0 end                       -- guard against clock going backwards
tokens = math.min(capacity, tokens + elapsed * rate / 1000.0)  -- refill, but never over max

local allowed = 0             -- assume denied
if tokens >= want then        -- enough tokens?
	tokens = tokens - want    -- spend them
	allowed = 1               -- allow the request
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)                     -- save new state
redis.call('PEXPIRE', KEYS[1], math.ceil(capacity / rate * 1000) + 1000)     -- auto-delete when idle long enough to be full

local retry = 0                                                       -- seconds to wait (0 if allowed)
if allowed == 0 then retry = math.ceil((want-tokens) / rate) end      -- time until enough tokens refill
return {allowed, math.floor(tokens), retry}                           -- {1/0, tokens left, retry secs}

-- Request 1: brand new user
-- capacity = 10
-- rate     = 2 -- (2 tokens per second)
-- want     = 1

-- now      = 1791455700000      -- 8 Oct 2026, 18:35:00 in ms

-- b        = {nil, nil}         -- key doesn't exist yet
-- tokens   = 10                 -- new bucket → full
-- ts       = 1791455700000      -- set to now

-- elapsed  = 0                  -- no time passed
-- tokens   = min(10, 10 + 0) = 10

-- HSET
-- ratelimit:user:42
--   tokens = 9
--   ts     = 1791455700000

-- PExpiry: 10 / 2 * 1000 + 1000 = 6000 ms (6 seconds).

-- allowed  = 1                  -- 10 >= 1 ✅
-- tokens   = 9                  -- spent 1

-- Returns: {1, 9, 0}, meaning allowed, 9 left, no wait.

----------
-- Request 2: bucket almost empty, user spamming
-- redis store
-- tokens = 0
-- ts     = 1791455703000

-- The next request arrives 200 ms later:
-- now      = 1791455703200
-- tokens   = 0
-- ts       = 1791455703000

-- elapsed  = 200                         -- 200 ms passed
-- tokens   = min(10, 0 + 200 * 2 / 1000)  = 0.4   -- refilled 0.4 tokens

-- allowed  = 0                  -- 0.4 < 1 ❌

-- What gets saved by HSET:
-- ratelimit:user:42
--   tokens = 0.4
--   ts     = 1791455703200

-- retry = ceil((1 - 0.4) / 2) = ceil(0.3) = 1

-- Example: capacity 5, refill 2.5/s):
-- req 1 -> 1 4 0        {allowed, tokens_left, retry_after}
-- req 2 -> 1 3 0
-- req 3 -> 1 2 0
-- req 4 -> 1 1 0
-- req 5 -> 1 0 0
-- req 6 -> 0 0 1        <- rejected, come back in 1s
-- req 7 -> 0 0 1
-- --- wait 1s (refills 2.5 tokens) ---
-- req 1 -> 1 1 0
-- req 2 -> 1 0 0
-- req 3 -> 0 0 1
