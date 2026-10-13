# Documentation

## Check all  the database, redis, and kafka
- make psql = psql "postgres://postgres:postgres@localhost:5432/
- make redis = redis-cli -h localhost -p 6379
Because both 5432 and 6379 have been port-forward [(Refer to this)](#port-forwards-that-tilt-sets-up-to-access-resources), so we can access then through our laptop localhost:5432 and localhost:6379

#### Postgres (Check the db and table)
user_db
```sh
make psql DB=user_db ARGS="-c 'SELECT id, phone, nick_name, balance, role FROM app_user;'"
make psql DB=user_db ARGS="-c 'SELECT * FROM follow;'"
```
Output:
1. app_user have 3 seeded users and 1 admin
```md
 id |    phone    |   nick_name   | balance | role
----+-------------+---------------+---------+-------
  1 | 0123456789  | Ahmad Faizal  |  100000 | user
  2 | 0198765432  | Tan Mei Ling  |  100000 | user
  3 | 01123456780 | Ravi Kumar    |       0 | user
  4 | 0100000000  | NearBuy Admin |       0 | admin
```
2. follow is empty


shop_db
```sh
make psql DB=shop_db ARGS="-c 'SELECT id, name, sort_order FROM shop_type ORDER BY sort_order;'"
make psql DB=shop_db ARGS="-c 'SELECT id, name, type_id, area, avg_price, sold, score FROM shop ORDER BY id;'"
make psql DB=shop_db ARGS="-c 'SELECT * FROM applied_event ORDER BY applied_at DESC LIMIT 20;'"
```
Output:
1. 10 shop_types
2. 14 shops
3. empty applied_event


voucher_db
```sh
make psql DB=voucher_db ARGS="-c 'SELECT id, shop_id, title, pay_value, actual_value, type, status FROM voucher ORDER BY id;'"
make psql DB=voucher_db ARGS="-c 'SELECT voucher_id, stock, begin_time, end_time FROM seckill_voucher;'"
make psql DB=voucher_db ARGS="-c 'SELECT * FROM stock_restore;'"
```
Output:
1. Empty voucher, seckill voucher and stock_restore


order_db
```sh
make psql DB=order_db ARGS="-c 'SELECT id, user_id, voucher_id, shop_id, pay_value, status, create_time, pay_time, stock_restored_at FROM voucher_order ORDER BY create_time DESC LIMIT 20;'"
```
Output:
1. Empty voucher order


dtm_db
```sh
make psql DB=dtm_db ARGS="-c 'SELECT gid, status, create_time, finish_time, rollback_reason FROM trans_global ORDER BY id DESC LIMIT 20;'"
make psql DB=dtm_db ARGS="-c 'SELECT gid, branch_id, op, status, url FROM trans_branch_op ORDER BY id DESC LIMIT 20;'"
```
Output: 
1. All of empty now
2. `trans_global` has one row per saga. 
	1. The gid shows which saga it is: 
		1. seckill-(id)  S1
		2. pay-(id)  S2
	2. status is succeed, failed or submitted
	3. (S3 expire saga is removed, now is a expiry job in order-service, [refer to this](#order-expiry-job))
3. `trans_branch_op` has one row per branch call (action or compensate).


DTM barrier tables (user_db, voucher_db, order_db)
```sh
make psql DB=user_db    ARGS="-c 'SELECT gid, branch_id, op, reason, create_time FROM dtm_barrier.barrier ORDER BY id DESC LIMIT 20;'"
make psql DB=voucher_db ARGS="-c 'SELECT gid, branch_id, op, reason, create_time FROM dtm_barrier.barrier ORDER BY id DESC LIMIT 20;'"
make psql DB=order_db   ARGS="-c 'SELECT gid, branch_id, op, reason, create_time FROM dtm_barrier.barrier ORDER BY id DESC LIMIT 20;'"
```
Output
user/voucher/order_db > dtm_barrier (schema) > barrier (table) are empty


Check Migrations Status
```sh
for db in user_db shop_db voucher_db order_db dtm_db; 
	make -s psql DB=$db ARGS="-c 'SELECT current_database(), version, dirty FROM schema_migrations;'"; 
end
```
Output：

| current_database | version | dirty |
|---|---|---|
| user_db | 5 | f |
| shop_db | 3 | f |
| voucher_db | 3 | f |
| order_db | 4 | f |
| dtm_db | 1 | f |
> make migrate-reset will reapplied the migration (migrate-down then migrate-up)

#### Redis (Check the redis status)
```sh
make redis ARGS=ping                     # expect PONG
make redis ARGS=dbsize                   # number of keys (1)
make redis ARGS="--scan --pattern '*'"   # list every key
# rl:ip:10.244.0.5 <- from the curl /api/ping just now
```

#### Kafka (Check all the topics)
```sh
kubectl exec kafka-0 -- /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
```
Output:
```md
__consumer_offsets
order.state_changed
order.state_changed.dlq
seckill.admitted
seckill.admitted.dlq
```
> __consumer_offsets is kafka internal topic, it appear after the consumer (order-service, shop-service) join


## User Service
`api/user/:id`
```sh
curl -s localhost/api/user/1 | jq
curl -i localhost/api/user/1
# {"id":1,"nickname":"Ahmad Faizal","icon":"/icons/1.png"}

curl -s localhost/api/user/999 | jq
curl -i localhost/api/user/999
# Output: user not found

curl -s localhost/api/user/abc | jq
curl -i localhost/api/user/abc
# Output: invalid user id
```


## Auth Service

`api/user/code`
- API Result : SMS Code for that phone number
- SMS Code Expiry Time: 2 minutes
- SMS Code Rate Limiting: every 1 minute only can access 1 code for 1 phone

`api/user/login`
- Register user if new phone number
- Get user if phone number already exist in db
- API Result: Session Token to access authenticated url
- Session Token Expiry Time: 30 minutes

`api/user/me`
- Provide the token as Bearer Header
- API Result: token user information

`api/user/logout`
- Logout the user by deleting session token  from redis -> login:token:(token)

Validation for the api
```sh
# a malformed phone is rejected
curl -s -X POST http://localhost/api/user/code \
  -H 'Content-Type: application/json' \
  -d '{"phone":"12345"}'
# {"message":"invalid phone number"}


# the code is single-use -- replay it
curl -s -X POST http://localhost/api/user/login \
  -H 'Content-Type: application/json' \
-d '{"phone": "0123456789", "code": "123456"}'

curl -s -X POST http://localhost/api/user/login \
  -H 'Content-Type: application/json' \
-d '{"phone": "0123456789", "code": "123456"}'
# {"message":"code is wrong or expired"} [401]


# a wrong code is indistinguishable from no code
curl -s -X POST http://localhost/api/user/login \
  -H 'Content-Type: application/json' \
  -d '{"phone": "0123456789", "code": "000000"}'
# {"message":"code is wrong or expired"} [401]

# No Token provide to /api/user/me
curl -s http://localhost/api/user/me
# {"message":"authentication required"} [401]

# Invalid Token provide to /api/user/me
curl -s -H 'Authorization: Bearer notatoken' http://localhost/api/user/me
# {"message":"authentication required"} [401]

# No Token provide to /api/user/logout
curl -s -X POST http://localhost/api/user/logout
# {"message":"authentication required"} [401]

# Invalid Token provide to /api/user/logout
curl -s -X POST -H 'Authorization: Bearer notatoken' http://localhost/api/user/logout
# {"message":"authentication required"} [401]
```

Login with Existing Phone User
```sh
make psql DB=user_db ARGS="-c 'SELECT id, phone, nick_name, balance FROM app_user;'"
# id | phone      | nick_name    | balance 
#  1 | 0123456789 | Ahmad Faizal | 100000

# Get exist user
curl -X POST http://localhost/api/user/code \
	 -H 'Content-Type: application/json' \
	 -d '{"phone":"0123456789"}'
# {"sent":true,"code":"759090"}


make redis ARGS="TTL 'login:code:0123456789'"
# 120 <- means 2 mins

make redis ARGS="GET 'login:code:0123456789'"
# 759090 <- sms code

# Login user 1
curl -X POST http://localhost/api/user/login \
     -H 'Content-Type: application/json' \
     -d '{"phone": "0123456789", "code": "759090"}'
# {"token":"OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4","user":{"id":1,"nickname":"Ahmad Faizal","icon":"/icons/1.png","role":"user"}}

make redis ARGS="GET 'login:code:0123456789'"
# nil means sms code get consume

make redis ARGS="TTL 'login:token:OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4'"    
# 1800 sec for 30 mins

make redis ARGS="GET 'login:token:OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4'"
# "{\"id\":1,\"nickname\":\"Ahmad Faizal\",\"icon\":\"/icons/1.png\",\"role\":\"user\"}"

# No New user will be created because is existing user
make psql DB=user_db ARGS="-c 'SELECT id, phone, nick_name, balance FROM app_user;'"
# id | phone      | nick_name    | balance 
#  1 | 0123456789 | Ahmad Faizal | 100000

# Provide token to get user info
curl http://localhost/api/user/me \
	 -H "Authorization: Bearer OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4"
# {"id":1,"nickname":"Ahmad Faizal","icon":"/icons/1.png","role":"user"}

# Logout
curl -X POST http://localhost/api/user/logout \
	 -H "Authorization: Bearer OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4"
	 
# Provide token to get user info
curl http://localhost/api/user/me \
	 -H "Authorization: Bearer OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4"
# {"message":"authentication required"} <- token deleted from redis

make redis ARGS="GET 'login:token:OxbRP99E-JfGIfvVUb9LZo8DITPLdQMTqXN0x3OGkF4'"
# nil <- token remove from redis
```

Login with New Phone User
```sh
# Create new user
curl -X POST http://localhost/api/user/code \
	 -H 'Content-Type: application/json' \
	 -d '{"phone":"0186666666"}'
# {"sent":true,"code":"156221"}

# Login new user (user 5, because admin is user 4)
curl -X POST http://localhost/api/user/login \
     -H 'Content-Type: application/json' \
     -d '{"phone": "0186666666", "code": "156221"}'

# Verify new user (user 5)
make psql DB=user_db ARGS="-c 'SELECT id, phone, nick_name, balance, role FROM app_user;'"
# Output
# id | phone      | nick_name     | balance | role
# 5  | 0186666666 | user_57on2pe9 | 0       | user
```

Login as Admin
```sh
curl -X POST http://localhost/api/user/code \
	 -H 'Content-Type: application/json' \
	 -d '{"phone":"0100000000"}'
# {"sent":true,"code":"840173"}

curl -X POST http://localhost/api/user/login \
     -H 'Content-Type: application/json' \
     -d '{"phone": "0100000000", "code": "840173"}'
# {"token":"TKQFwxHLb0a_EYWxAf-JpWLfNsYA9micqebU7bGiYyw","user":{"id":4,"nickname":"NearBuy Admin","icon":"/icons/admin.png","role":"admin"}}
```

Role
- admin: create/edit shop, create voucher. Cannot grab, pay or follow
- user (customer): grab, pay, follow. Cannot create/edit shop or voucher
```sh
# user try to edit shop
curl -s -X PATCH localhost/api/shops/1 -H "Authorization: Bearer <user token>" \
     -H 'Content-Type: application/json' -d '{"name":"x"}'
# {"message":"admin only"} [403]

# admin try to follow
curl -s -X PUT localhost/api/users/1/follow -H "Authorization: Bearer <admin token>"
# {"message":"admins cannot do this"} [403]
```


## Rate-Limiting
**Key**: rl:ip:(ip)
**Purpose**: Stops one client flooding the API. Applies to every /api/* request, logged in or public
**Limit Time**: 100 try per min
**Example**:
```sh
# fire 103 at a 100-capacity bucket (/api/ping is public, no token needed)
for i in (seq 1 103)
    curl -s -o /dev/null -w '%{http_code} ' http://localhost/api/ping
end
echo
# 200 200 200 ... (100 times) ... 200 429 429 429

make redis ARGS="--scan --pattern 'rl:ip:*'"
# "rl:ip:10.244.0.5"

curl -s -D - -o /dev/null http://localhost/api/ping | grep -i ratelimit
# X-Ratelimit-Limit: 100
# X-Ratelimit-Remaining: 5
```


**Key**: rl:user:(id):(METHOD):(path)
**Purpose**: Stops one logged-in user spam one endpoint. Each route has its own bucket for a logged-in user
**Limit Time**: 10 try per min
**Example**:
```sh
# fire 13 at a 10-capacity bucket
for i in (seq 1 13)
    curl -s -o /dev/null -w '%{http_code} ' -H "Authorization: Bearer 2i8zTohc7TEZiBFPng0kHk2NMUe3ajwVVXeXZVtaaGc" http://localhost/api/user/me
end
echo
# 200 200 200 200 200 200 200 200 200 200 429 429 429

make redis ARGS="--scan --pattern 'rl:user:*'"
# "rl:user:1:GET:/api/user/me"
```



**Key**: rl:sms:(phone)
**Purpose**: Stops SMS spam to one phone number.
**Limit Time**: 1 try per min
**Example**:
```sh
curl -X POST http://localhost/api/user/code \
	 -H 'Content-Type: application/json' \
	 -d '{"phone":"0123456789"}'
# {"sent":true,"code":"664215"}

curl -X POST http://localhost/api/user/code \
	 -H 'Content-Type: application/json' \
	 -d '{"phone":"0123456789"}'
# {"message":"rate limit exceeded: per-phone-sms"} 
	 
make redis ARGS="TTL 'rl:sms:0123456789'"
# 60 <- means 1 mins

make redis ARGS="HGETALL rl:sms:0123456789"
#  1) "tokens"
#  2) "0" (Tokens left after the last request, used to only one when sent the code)

#  3) "ts"
#  4) "1791824645892"   (Time of that request, Tue 13 Oct 2026 01:04:05.892)
```


**Key**: rl:login:(phone)
**Purpose**: Stops brute-forcing a 6-digit code. Limits login attempts per phone.
**Limit Time**: 5 try per 10 min
**Example**:
```sh
# fire 13 login attempt to /api/user/login
for i in (seq 1 13)
	curl -s -o /dev/null -w '%{http_code} ' -X POST http://localhost/api/user/login \
     -H 'Content-Type: application/json' \
     -d '{"phone": "0123456789", "code": "759090"}'
end
echo
# 401 401 401 401 401 429 429 429 429 429 429 429 429

make redis ARGS="--scan --pattern 'rl:login:*'"
# "rl:login:0123456789"
```


## Shop Service
```sh
make psql DB=shop_db ARGS="-c 'SELECT id, name, sort_order FROM shop_type ORDER BY sort_order;'"
# id |    name     | sort_order
#----+-------------+------------
#  1 | Food        |          1
#  2 | KTV         |          2
#  3 | Hair Salon  |          3
# 10 | Nails       |          4
#  5 | Massage     |          5
#  6 | Beauty SPA  |          6
#  7 | Kids Play   |          7
#  8 | Bar         |          8
#  9 | Party House |          9
#  4 | Gym         |         10

make psql DB=shop_db ARGS="-c 'SELECT id, name, type_id, area, avg_price, sold, score FROM shop ORDER BY id;'"
# id | name               | type_id | area   | avg_price | sold | score 
# 1  | 103 Tea Restaurant | 1       | Daguan | 80        | 4215 | 37
# ...


curl -s http://localhost/api/shops/1 | jq -c '{id, name, area}'
# {"id":1,"name":"103 Tea Restaurant","area":"Daguan"}

curl -s 'http://localhost/api/shops?type_id=1&page=1&page_size=3' | jq -c '{total, names: [.shops[].name]}'
# {"total":9,"names":["103 Tea Restaurant","Caima Grill Hotpot","Xin Bai Lu"]}

curl -s 'http://localhost/api/shops?name=KTV' | jq -c '[.shops[].name]'
# ["Kaledi KTV","INLOVE KTV","Mei KTV","OuK La KTV","Xingjuhui KTV"]

curl -s http://localhost/api/shop-types | jq -r '.[0:3][] | .name'
# Food
# KTV
# Hair Salon
```

The postgres result will store in redis
```sh
make redis ARGS="FLUSHALL"
curl -s -o /dev/null http://localhost/api/shops/1
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/api/shops/999999   # 404
curl -s -o /dev/null http://localhost/api/shop-types

make -s redis ARGS="GET cache:shop:1" | head -c 60
# {"id":1,"name":"103 Tea Restaurant","type_id":1,"images":"/i

make -s redis ARGS="TTL cache:shop:1"
# 1800 - 30 minutes

make -s redis ARGS="GET cache:shop:999999"
# "__nil__" - avoids cache penetration (without it, someone could keep requesting shop 999999 and every request would hit Postgres)

make -s redis ARGS="TTL cache:shop:999999"
# 120 - 2 minutes
# the nil entry has a shorter TTL because that shop may be created later

make -s redis ARGS="GET cache:shop-types" | head -c 60
# [{"id":1,"name":"Food","icon":"/types/ms.png","sort_order":1

make -s redis ARGS="TTL cache:shop-types"
# 86400 - 24 hours (shop types almost never change)
```
> FLUSHALL also delete the seckill:stock:* key, voucher-service only warm it on startup or create seckill voucher

Test Cache when postgers is down
```sh
# Set the result (shop 1 and shop 999999) to cache 
curl -s -o /dev/null http://localhost/api/shops/1
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/api/shops/999999   # 404

# Scale the postgres to 0
kubectl scale statefulset/postgres --replicas=0
kubectl wait --for=delete pod/postgres-0 --timeout=60s

# got cache, no db
curl -i http://localhost/api/shops/1
# 200 ok

# got cache, no db
curl -i http://localhost/api/shops/999999
# 404 ok

# no cache, no db
curl -i http://localhost/api/shops/7
# 500 due to postgres down and no cache

# no cache, no db
curl -i http://localhost/api/shops/888888
# 500 due to postgres down and no cache

kubectl scale statefulset/postgres --replicas=1
```

Cache Invalidation when updating the shop to new value
- It will update the db, then delete the cache (cache-aside problem - consistency between db and cache)
- Only admin can update shop
```sh
# Get admin token
curl -X POST http://localhost/api/user/code \
	 -H 'Content-Type: application/json' \
	 -d '{"phone":"0100000000"}'
# {"sent":true,"code":"774612"}
curl -X POST http://localhost/api/user/login \
     -H 'Content-Type: application/json' \
     -d '{"phone": "0100000000", "code": "774612"}'
# {"token":"TKQFwxHLb0a_EYWxAf-JpWLfNsYA9micqebU7bGiYyw","user":{"id":4,"nickname":"NearBuy Admin","icon":"/icons/admin.png","role":"admin"}}

# Store in Cache
curl -s http://localhost/api/shops/1
redis-cli -h localhost -p 6379 GET cache:shop:1 | jq -r .name     
# 103 Tea Restaurant

curl -s -X PATCH http://localhost/api/shops/1 \
     -H "Authorization: Bearer TKQFwxHLb0a_EYWxAf-JpWLfNsYA9micqebU7bGiYyw" -H 'Content-Type: application/json' \
     -d '{"name":"103 Tea Restaurant (renamed)"}'
# [204]

make redis ARGS="EXISTS cache:shop:1"
# 0 <- cache get deleted due to update do the cache invalidation

curl -s http://localhost/api/shops/1 | jq -r '.name, .address'
# 103 Tea Restaurant (renamed)
# 29 Jinhua Rd                 <- untouched by the partial update

make redis ARGS="EXISTS cache:shop:1"
# 1 <- previous call store the shop 1 to cache
```

Cache Invalidation when create the new shop
- It will update the db, then delete the cache (cache-aside problem - consistency between db and cache)
```sh
# Store not found value in cache
curl -s -o /dev/null -w '%{http_code}\n' http://localhost/api/shops/15     
# 404
redis-cli -h localhost -p 6379 GET cache:shop:15                   # __nil__

curl -s -X POST http://localhost/api/shops -H "Authorization: Bearer TKQFwxHLb0a_EYWxAf-JpWLfNsYA9micqebU7bGiYyw" \
     -H 'Content-Type: application/json' \
     -d '{"name":"Brand New Cafe","type_id":1,"address":"1 New St","x":120.15,"y":30.32,"avg_price":50}' \
  | jq -c '{id, name}'
# {"id":15,"name":"Brand New Cafe"}

redis-cli -h localhost -p 6379 EXISTS cache:shop:15                # 0 <- cache get deleted due to update do the cache invalidation

curl -s -w '%{http_code}\n' http://localhost/api/shops/15     
# 200, not a stale 404
```

Lists are not cached
```sh
curl -s -o /dev/null 'http://localhost/api/shops?type_id=1'
make redis ARGS="--scan --pattern 'cache:shop:list*'"
# nothing
```

## Follow Service
- Postgres `follow` table is the source of truth, Redis `follows:(userId)` is a SET mirror
- Follow: write postgres first, then SADD redis
- Common following: SINTER follows:(me) follows:(other), fallback to SQL if redis down
- Only customer can follow, actor always the login user (token), not the path

Token used below
- U1 = Ahmad Faizal (user 1)
- U2 = Tan Mei Ling (user 2)

```sh
# user 1 follow user 2 and user 3
curl -s -w '%{http_code}\n' -X PUT localhost/api/users/2/follow -H "Authorization: Bearer $U1"
# 204
curl -s -w '%{http_code}\n' -X PUT localhost/api/users/3/follow -H "Authorization: Bearer $U1"
# 204

# follow again, still 204 (idempotent)
curl -s -w '%{http_code}\n' -X PUT localhost/api/users/2/follow -H "Authorization: Bearer $U1"
# 204

# follow yourself
curl -s -X PUT localhost/api/users/1/follow -H "Authorization: Bearer $U1"
# {"message":"cannot follow yourself"} [400]

# user 2 follow user 3
curl -s -w '%{http_code}\n' -X PUT localhost/api/users/3/follow -H "Authorization: Bearer $U2"
# 204

# Follow counts (public)
curl -s localhost/api/users/1/follow-counts
# {"following":2,"followers":0}
curl -s localhost/api/users/3/follow-counts
# {"following":0,"followers":2}

# Is user 1 following user 2?
curl -s localhost/api/users/2/follow -H "Authorization: Bearer $U1"
# {"following":true}
curl -s localhost/api/users/1/follow -H "Authorization: Bearer $U2"
# {"following":false}

# Who user 1 and user 2 both follow
curl -s localhost/api/users/2/common-following -H "Authorization: Bearer $U1"
# [{"id":3,"nickname":"Ravi Kumar","icon":"/icons/3.png"}]
```

Check the postgres and redis
```sh
make psql DB=user_db ARGS="-c 'SELECT user_id, follow_user_id FROM follow;'"
#  user_id | follow_user_id
# ---------+----------------
#        1 |              2
#        1 |              3
#        2 |              3

make redis ARGS="--scan --pattern 'follows:*'"
# follows:1
# follows:2

make redis ARGS="SMEMBERS follows:1"
# 2
# 3

make redis ARGS="TTL follows:1"
# -1 <- no expiry, it is a mirror not a cache
```

Unfollow
```sh
curl -s -w '%{http_code}\n' -X DELETE localhost/api/users/3/follow -H "Authorization: Bearer $U2"
# 204

curl -s localhost/api/users/2/common-following -H "Authorization: Bearer $U1"
# [] <- user 2 no longer follow user 3
```
> Follow a user id that not exist (e.g. 999) also return 204, no check the user exist


## FlashSale
Flow
1. Admin create seckill voucher -> voucher-service save to voucher_db and warm `seckill:stock:(vid)` in redis
2. Customer grab -> voucher-service run lua script in redis (check time, stock, already ordered)
3. Admitted -> publish `seckill.admitted` to kafka, reply 202 with order_id (order not exist yet)
4. order-service consume -> start Saga S1 (create order, deduct stock)

Create voucher (admin)
```sh
# Normal voucher
curl -s -X POST localhost/api/vouchers -H "Authorization: Bearer $AD" -H 'Content-Type: application/json' \
     -d '{"shop_id":1,"title":"RM50 Voucher","sub_title":"Mon-Fri","rules":"Dine-in only","pay_value":4000,"actual_value":5000}'
# {"id":1,"shop_id":1,"title":"RM50 Voucher",...,"type":"normal","status":"active"} [201]

# Seckill voucher, stock 2, sale start now
curl -s -X POST localhost/api/vouchers/seckill -H "Authorization: Bearer $AD" -H 'Content-Type: application/json' \
     -d '{"shop_id":1,"title":"RM100 Flash Deal","sub_title":"Limited","rules":"One per user","pay_value":5000,"actual_value":10000,"stock":2,"begin_time":"2026-10-12T22:26:28Z","end_time":"2026-10-13T00:27:28Z"}'
# {"id":2,...,"type":"seckill","status":"active","stock":2,"begin_time":"2026-10-12T22:26:28Z","end_time":"2026-10-13T00:27:28Z"} [201]

# Voucher 3 = sale start in 1 hour, Voucher 4 = sale already ended
```

```sh
curl -s localhost/api/shops/1/vouchers | jq -c '.[] | {id,title,type,stock}'
# {"id":1,"title":"RM50 Voucher","type":"normal","stock":null}
# {"id":2,"title":"RM100 Flash Deal","type":"seckill","stock":2}
# {"id":3,"title":"Future Deal","type":"seckill","stock":5}
# {"id":4,"title":"Ended Deal","type":"seckill","stock":5}

make redis ARGS="GET seckill:stock:2"
# 2
```

Grab (customer)
```sh
# user 1 grab
curl -s -X POST localhost/api/voucher-order/seckill/2 -H "Authorization: Bearer $U1"
# {"order_id":241181560634605569,"status":"pending"} [202]

# user 1 grab again
curl -s -X POST localhost/api/voucher-order/seckill/2 -H "Authorization: Bearer $U1"
# {"message":"already ordered"} [409]

# user 2 grab (last stock)
curl -s -X POST localhost/api/voucher-order/seckill/2 -H "Authorization: Bearer $U2"
# {"order_id":241181564929572866,"status":"pending"} [202]

# user 3 grab
curl -s -X POST localhost/api/voucher-order/seckill/2 -H "Authorization: Bearer $U3"
# {"message":"out of stock"} [409]

curl -s -X POST localhost/api/voucher-order/seckill/3 -H "Authorization: Bearer $U1"
# {"message":"sale has not started"} [425]

curl -s -X POST localhost/api/voucher-order/seckill/4 -H "Authorization: Bearer $U1"
# {"message":"sale has ended"} [410]

curl -s -X POST localhost/api/voucher-order/seckill/99 -H "Authorization: Bearer $U1"
# {"message":"unknown voucher"} [404]

curl -s -X POST localhost/api/voucher-order/seckill/2 -H "Authorization: Bearer $AD"
# {"message":"admins cannot do this"} [403]
```

Check redis
```sh
make redis ARGS="GET seckill:stock:2"
# 0

make redis ARGS="SMEMBERS seckill:order:2"
# 1
# 2   <- user id that already admitted

make redis ARGS="--scan --pattern 'icr:*'"
# icr:order:2026:10:12 <- id generator for order_id (per day counter)
```

Check db (after S1 done)
```sh
make psql DB=order_db ARGS="-c 'SELECT id, user_id, voucher_id, shop_id, pay_value, status FROM voucher_order;'"
#          id         | user_id | voucher_id | shop_id | pay_value | status
# --------------------+---------+------------+---------+-----------+--------
#  241181560634605569 |       1 |          2 |       1 |      5000 | unpaid
#  241181564929572866 |       2 |          2 |       1 |      5000 | unpaid

make psql DB=voucher_db ARGS="-c 'SELECT voucher_id, stock FROM seckill_voucher;'"
#  voucher_id | stock
# ------------+-------
#           2 |     0   <- redis and postgres both 0
#           3 |     5
#           4 |     5
```

Order api
```sh
curl -s localhost/api/voucher-orders -H "Authorization: Bearer $U1"
# [{"id":241181560634605569,"user_id":1,"voucher_id":2,"pay_value":5000,"pay_type":"balance","status":"unpaid","create_time":"2026-10-12T22:27:39Z"}]

# user 1 try to see user 2 order
curl -s localhost/api/voucher-order/241181564929572866 -H "Authorization: Bearer $U1"
# {"message":"order not found"} [404]
```


## Kafka
| Topic                   | Producer        | Consumer (group)                        | What it do                     |
| ----------------------- | --------------- | --------------------------------------- | ------------------------------ |
| seckill.admitted        | voucher-service | order-service (order-service.saga-starter) | start Saga S1                  |
| order.state_changed     | order-service   | shop-service (shop-service.shop-stats)  | shop.sold + 1 when order paid  |
| *.dlq                   | consumer        | nobody                                  | poison message park here       |

```sh
kubectl exec kafka-0 -- /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list
# order-service.saga-starter
# shop-service.shop-stats
```

Read the message
```sh
kubectl exec kafka-0 -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 \
  --topic seckill.admitted --from-beginning --timeout-ms 5000
# {"order_id":241181560634605569,"user_id":1,"voucher_id":2,"shop_id":1,"pay_value":5000}
# {"order_id":241181564929572866,"user_id":2,"voucher_id":2,"shop_id":1,"pay_value":5000}

kubectl exec kafka-0 -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 \
  --topic order.state_changed --from-beginning --timeout-ms 5000
# {"order_id":241181560634605569,"shop_id":1,"status":"paid","user_id":1,"voucher_id":2}
```

shop.sold + 1 after order paid
```sh
make psql DB=shop_db ARGS="-t -c 'SELECT sold FROM shop WHERE id=1'"
# 4215 -> 4216 after user 1 pay

make psql DB=shop_db ARGS="-c 'SELECT * FROM applied_event'"
#                   event_key                  |        topic
# ---------------------------------------------+---------------------
#  order.state_changed:241181560634605569:paid | order.state_changed
```

Duplicate message (idempotent consumer)
- kafka deliver at least once, so same message can come twice
- `applied_event` is the done list, already there -> skip
```sh
# kafkagen publish order.state_changed by hand (laptop need "127.0.0.1 kafka" in /etc/hosts)
cd services/shop-service
go run ./cmd/kafkagen -order 241181560634605569 -shop 1 -status paid
# published order_id=241181560634605569 status=paid trace_id=58930369-...

make psql DB=shop_db ARGS="-t -c 'SELECT sold FROM shop WHERE id=1'"
# 4216 <- not 4217

kubectl logs deploy/shop-service | grep duplicate
# "msg":"duplicate event ignored","event_key":"order.state_changed:241181560634605569:paid"
```

Poison message (DLQ)
```sh
go run ./cmd/kafkagen -raw 'not json'

kubectl logs deploy/shop-service | grep -i 'dead\|permanent'
# "msg":"handler failed",...,"permanent":true,"error":"permanent failture: unmarshal: invalid character 'o' ..."
# "msg":"message dead-lettered","topic":"order.state_changed","dlq":"order.state_changed.dlq","attempts":1

kubectl exec kafka-0 -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 \
  --topic order.state_changed.dlq --from-beginning --timeout-ms 5000 --property print.headers=true
# attempt:1,error:permanent failture: ...,origin-topic:order.state_changed,origin-partition:1,origin-offset:0,trace-id:a7209ee2-...	not json
```
> permanent error (bad json) go DLQ straight away, transient error (db down) retry first then DLQ


## Distributed Transaction Saga
DTM console: http://localhost:36789

#### S1 - Seckill (create order -> deduct stock)
- gid = seckill-(order_id), same message come twice still one saga
- create_order first, deduct_stock last (scarce resource go last)
```sh
make psql DB=dtm_db ARGS="-c 'SELECT gid, status FROM trans_global;'"
#             gid             | status
# ----------------------------+---------
#  seckill-241181560634605569 | succeed
#  seckill-241181564929572866 | succeed

make psql DB=dtm_db ARGS="-c 'SELECT gid, branch_id, op, status, url FROM trans_branch_op ORDER BY id;'"
#             gid             | branch_id |     op     |  status  |                         url
# ----------------------------+-----------+------------+----------+------------------------------------------------------
#  seckill-241181560634605569 | 01        | compensate | prepared | http://order-service:8000/branch/create_order_comp
#  seckill-241181560634605569 | 01        | action     | succeed  | http://order-service:8000/branch/create_order
#  seckill-241181560634605569 | 02        | compensate | prepared | http://voucher-service:8000/branch/deduct_stock_comp
#  seckill-241181560634605569 | 02        | action     | succeed  | http://voucher-service:8000/branch/deduct_stock
```
> compensate stay prepared because no need to run

#### S1 Saga Demo (cmd/sagademo)
- Submit S1 by hand, no seckill, no kafka
- Branch base url is cluster DNS because DTM (inside cluster) call it, not the laptop
- DTM_SERVER is localhost through the 36789 port-forward
```sh
# run at nearbuy root
set -x DTM_SERVER          http://localhost:36789/api/dtmsvr
set -x ORDER_BRANCH_BASE   http://order-service:8000/branch
set -x VOUCHER_BRANCH_BASE http://voucher-service:8000/branch

# use voucher 3 (stock 5)
make psql DB=voucher_db ARGS="-tAc 'select stock from seckill_voucher where voucher_id=3'"
# 5
```

1. Happy path
```sh
go run ./cmd/sagademo -saga s1 -order-id 900001 -user 1 -voucher-id 3 -pay 1000 -wait
# submitting gid=seckill-900001
# submitted

make psql DB=order_db ARGS="-c 'select id,user_id,voucher_id,pay_value,status from voucher_order where id < 1000000'"
#    id   | user_id | voucher_id | pay_value | status
# --------+---------+------------+-----------+--------
#  900001 |       1 |          3 |      1000 | unpaid

make psql DB=voucher_db ARGS="-tAc 'select stock from seckill_voucher where voucher_id=3'"
# 4 <- 5 -> 4, two db one saga

make psql DB=dtm_db ARGS="-c \"select branch_id,op,status from trans_branch_op where gid='seckill-900001' order by branch_id,op\""
#  branch_id |     op     |  status
# -----------+------------+----------
#  01        | action     | succeed
#  01        | compensate | prepared
#  02        | action     | succeed
#  02        | compensate | prepared

# barrier, one row per branch in its own db
make psql DB=order_db   ARGS="-c \"select gid,branch_id,op,barrier_id,reason from dtm_barrier.barrier where gid='seckill-900001'\""
#  seckill-900001 | 01 | action | 01 | action
make psql DB=voucher_db ARGS="-c \"select gid,branch_id,op,barrier_id,reason from dtm_barrier.barrier where gid='seckill-900001'\""
#  seckill-900001 | 02 | action | 01 | action
```

2. Same gid again
```sh
go run ./cmd/sagademo -saga s1 -order-id 900001 -user 1 -voucher-id 3 -pay 1000 -wait
# submitting gid=seckill-900001
# submit error: {"dtm_result":"FAILURE","message":"current status 'succeed', cannot sumbmit. FAILURE"}

make psql DB=voucher_db ARGS="-tAc 'select stock from seckill_voucher where voucher_id=3'"
# 4 <- DTM refuse, no branch run
```

3. The bug: same order with random gid
```sh
go run ./cmd/sagademo -saga s1 -order-id 900001 -user 1 -voucher-id 3 -pay 1000 -gid (uuidgen) -wait
# submitting gid=66300D92-1C52-4AA8-835D-973F6205B864
# submitted

make psql DB=order_db   ARGS="-tAc 'select count(*) from voucher_order where id=900001'"
# 1
make psql DB=voucher_db ARGS="-tAc 'select stock from seckill_voucher where voucher_id=3'"
# 3 <- one order, stock deduct twice
```
> new gid = new barrier key, so barrier cannot stop it. That why gid = seckill-(order_id), not random

```sh
# reset
make psql DB=voucher_db ARGS="-c 'update seckill_voucher set stock = 4 where voucher_id = 3'"
```

4. Rollback: out of stock
```sh
# deduct 100 from stock 4
go run ./cmd/sagademo -saga s1 -order-id 900002 -user 2 -voucher-id 3 -pay 1000 -qty 100 -wait
# submitting gid=seckill-900002
# submit error: {"dtm_result":"FAILURE","message":"url:http://voucher-service:8000/branch/deduct_stock return failed: {\"error\":\"FAILURE\"} FAILURE"}

make psql DB=order_db ARGS="-c 'select id,user_id,status from voucher_order where id < 1000000 order by id'"
#    id   | user_id | status
# --------+---------+--------
#  900001 |       1 | unpaid    <- order 900002 is deleted (not cancelled)

make psql DB=voucher_db ARGS="-tAc 'select stock from seckill_voucher where voucher_id=3'"
# 4 <- untouched

make psql DB=dtm_db ARGS="-c \"select gid,status,rollback_reason from trans_global where gid='seckill-900002'\""
#       gid       | status |                rollback_reason
# ----------------+--------+------------------------------------------------------------------------
#  seckill-900002 | failed | url:http://voucher-service:8000/branch/deduct_stock return failed: ...

make psql DB=dtm_db ARGS="-c \"select branch_id,op,status from trans_branch_op where gid='seckill-900002' order by branch_id,op\""
#  branch_id |     op     | status
# -----------+------------+---------
#  01        | action     | succeed
#  01        | compensate | succeed   <- delete the order
#  02        | action     | failed
#  02        | compensate | succeed
```

Barrier
```sh
make psql DB=order_db ARGS="-c \"select branch_id,op,barrier_id,reason from dtm_barrier.barrier where gid='seckill-900002' order by id\""
#  branch_id |     op     | barrier_id |   reason
# -----------+------------+------------+------------
#  01        | action     | 01         | action
#  01        | compensate | 01         | compensate

make psql DB=voucher_db ARGS="-c \"select branch_id,op,barrier_id,reason from dtm_barrier.barrier where gid='seckill-900002' order by id\""
#  branch_id |     op     | barrier_id |   reason
# -----------+------------+------------+------------
#  02        | action     | 01         | compensate   <- empty compensation
#  02        | compensate | 01         | compensate
```
- order_db: action commit, so compensate run (delete order)
- voucher_db: action failed and rolled back (barrier row also gone). compensate insert the action key first and success, means action never commit, so skip the compensate body. Stock stay 4, not 104

5. Kill voucher-service mid-saga
```sh
kubectl scale deploy/voucher-service --replicas=0
go run ./cmd/sagademo -saga s1 -order-id 900003 -user 3 -voucher-id 3 -pay 1000
# submitted

make psql DB=order_db ARGS="-tAc 'select id,status from voucher_order where id=900003'"
# 900003|unpaid
make psql DB=dtm_db ARGS="-tAc \"select status from trans_global where gid='seckill-900003'\""
# submitted
make psql DB=dtm_db ARGS="-c \"select branch_id,op,status from trans_branch_op where gid='seckill-900003' order by branch_id,op\""
#  01        | action     | succeed
#  01        | compensate | prepared
#  02        | action     | prepared   <- voucher-service down, DTM keep retry (not rollback)
#  02        | compensate | prepared

kubectl scale deploy/voucher-service --replicas=1
# wait one retry interval (10s)

make psql DB=dtm_db ARGS="-tAc \"select status from trans_global where gid='seckill-900003'\""
# succeed
make psql DB=voucher_db ARGS="-tAc 'select stock from seckill_voucher where voucher_id=3'"
# 3 <- saga finish by itself
```
> service down = ErrOngoing (retry), out of stock = ErrFailure (rollback)

#### S2 - Pay (deduct balance -> mark order paid)
- gid = pay-(order_id), double click pay still one saga
- balance first, order second (order paid but money not deducted is worse)
- request wait until saga finish

Success
```sh
curl -s -X POST localhost/api/voucher-order/241181560634605569/pay -H "Authorization: Bearer $U1"
# {"id":241181560634605569,...,"status":"paid","pay_time":"2026-10-12T22:27:54Z"} [200]

# pay again
curl -s -X POST localhost/api/voucher-order/241181560634605569/pay -H "Authorization: Bearer $U1"
# {"message":"order is not awaiting payment"} [409]

make psql DB=user_db ARGS="-t -c 'SELECT id, balance FROM app_user WHERE id=1'"
# 1 | 95000 <- 100000 - 5000
```

Rollback (balance not enough)
```sh
# set user 2 balance to 1000, order is 5000
make psql DB=user_db ARGS="-c 'UPDATE app_user SET balance=1000 WHERE id=2'"

curl -s -X POST localhost/api/voucher-order/241181564929572866/pay -H "Authorization: Bearer $U2"
# {"message":"payment failed and was rolled back"} [402]

make psql DB=dtm_db ARGS="-c 'SELECT gid, status, rollback_reason FROM trans_global ORDER BY id;'"
#            gid             | status  | rollback_reason
# ---------------------------+---------+------------------------------------------------------------------
#  pay-241181560634605569    | succeed |
#  pay-241181564929572866    | failed  | url:http://user-service:8000/branch/deduct_balance return failed: {"error":"FAILURE"} FAILURE

make psql DB=dtm_db ARGS="-c \"SELECT gid, branch_id, op, status FROM trans_branch_op WHERE gid like 'pay-%' ORDER BY id;\""
#           gid           | branch_id |     op     |  status
# ------------------------+-----------+------------+----------
#  pay-241181564929572866 | 01        | compensate | succeed
#  pay-241181564929572866 | 01        | action     | failed
#  pay-241181564929572866 | 02        | compensate | prepared
#  pay-241181564929572866 | 02        | action     | prepared   <- never run

make psql DB=order_db ARGS="-c 'SELECT id, user_id, status FROM voucher_order;'"
# 241181564929572866 | 2 | unpaid <- still unpaid, can pay after top up
```

Barrier
```sh
make psql DB=user_db ARGS="-c 'SELECT gid, branch_id, op, reason FROM dtm_barrier.barrier ORDER BY id'"
#           gid           | branch_id |     op     |   reason
# ------------------------+-----------+------------+------------
#  pay-241181560634605569 | 01        | action     | action
#  pay-241181564929572866 | 01        | action     | compensate
#  pay-241181564929572866 | 01        | compensate | compensate
```
> second pay saga: action failed, compensate still run. compensate insert an action row with reason compensate, so if the action arrive late (after compensate) it will be skipped

#### Order Expiry Job
- Old S3 saga is removed, now is a background job in order-service
- Every 30s (EXPIRY_INTERVAL), cancel unpaid order older than 15m (ORDER_EXPIRY)
- Then call voucher-service RestoreStock, idempotent by `stock_restore` table
```sh
# make user 2 order old (no need wait 15 mins)
make psql DB=order_db ARGS="-c \"UPDATE voucher_order SET create_time = now() - interval '16 minutes' WHERE status='unpaid'\""

# wait 30s
make psql DB=order_db ARGS="-c 'SELECT id, user_id, status, stock_restored_at FROM voucher_order;'"
#          id         | user_id |  status   |       stock_restored_at
# --------------------+---------+-----------+-------------------------------
#  241181560634605569 |       1 | paid      |
#  241181564929572866 |       2 | cancelled | 2026-10-12 22:28:59.284833+00

make psql DB=voucher_db ARGS="-c 'SELECT * FROM stock_restore;'"
#       order_id      | voucher_id |          create_time
# --------------------+------------+-------------------------------
#  241181564929572866 |          2 | 2026-10-12 22:28:59.280678+00

make psql DB=voucher_db ARGS="-t -c 'SELECT stock FROM seckill_voucher WHERE voucher_id=2'"
# 1 <- stock back

make redis ARGS="GET seckill:stock:2"
# 1
make redis ARGS="SMEMBERS seckill:order:2"
# 1 <- user 2 removed, can grab again

kubectl logs deploy/order-service | grep -i 'expir\|restor'
# "msg":"expiry scheduler started","interval":30000000000,"expiry":900000000000,"batch":100
# "msg":"cancelled expired orders","count":1
# "msg":"expired order stock restored","order_id":241181564929572866,"voucher_id":2,"restored_now":true
```



## API Schema
#### Auth & User

| Method & Path           | Access | Input                                           | Success response                                                                                    |
| ----------------------- | ------ | ----------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `GET /api/ping`         | Public | –                                               | 200 `{"message":"pong","pod":"..."}`                                                                |
| `POST /api/user/code`   | Public | body <br>{"phone":"0123456789"}                 | 200 `{"sent":true,"code":"123456"}`. `code` is only returned when `DEV_ECHO_SMS=true`.              |
| `POST /api/user/login`  | Public | body {"phone":"0123456789",<br>"code":"123456"} | 200 `{"token":"...","user":{"id","nickname","icon","role"}}`. Creates the user if the phone number is new. |
| `GET /api/user/me`      | Auth   | –                                               | 200 `{"id","nickname","icon","role"}`                                                               |
| `POST /api/user/logout` | Auth   | –                                               | 204                                                                                                 |
| `GET /api/user/:id`     | Public | path `id`                                       | 200 `{"id","nickname","icon"}`                                                                      |

Phone format: Malaysian mobile, either `01X` followed by 7 digits or `011` followed by 8 digits.

| Route | Status | Reason |
|---|---|---|
| `/user/code` | 400 | bad phone number |
| `/user/code` | 429 | `rl:sms` limit |
| `/user/login` | 400 | phone or code is missing |
| `/user/login` | 401 | "code is wrong or expired" |
| `/user/login` | 429 | `rl:login` limit |

Access
- Auth = any login user
- Admin = role admin, else 403 `admin only`
- Customer = role user, admin get 403 `admins cannot do this`

#### Follow

| Method & Path                         | Access   | Input                                                       | Success response                      |
| ------------------------------------- | -------- | ----------------------------------------------------------- | ------------------------------------- |
| `GET /api/users/:id/follow-counts`    | Public   | path `id`                                                   | 200 `{"following":n,"followers":n}`   |
| `PUT /api/users/:id/follow`           | Customer | path `id` (the user to follow)                              | 204                                   |
| `DELETE /api/users/:id/follow`        | Customer | path `id`<br>(the user to unfollow)                         | 204                                   |
| `GET /api/users/:id/follow`           | Customer | path `id`<br>(the other user that login user are following) | 200 `{"following":true}`              |
| `GET /api/users/:id/common-following` | Customer | path `id` (the other user)                                  | 200 `[{"id","nickname","icon"}, ...]` |

- the login user will compare to the path id user
#### Shop

| Method & Path | Access | Input | Success response |
|---|---|---|---|
| `GET /api/shop-types` | Public | – | 200 `[{"id","name","icon","sort_order"}, ...]` |
| `GET /api/shops` | Public | query `type_id` or `name` (one is required), plus `page` (default 1) and `page_size` (default 10) | 200 `{"shops":[Shop...],"total":n}` |
| `GET /api/shops/:id` | Public | path `id` | 200 Shop |
| `POST /api/shops` | Admin | body `{"name","type_id","images","area","address","x","y","avg_price","open_hours"}` | 201 Shop |
| `PATCH /api/shops/:id` | Admin | body: any subset of `name`, `type_id`, `images`, `area`, `address`, `avg_price`, `open_hours`. Fields you leave out stay unchanged. | 204 |


#### Voucher

| Method & Path | Access | Input | Success response |
|---|---|---|---|
| `GET /api/vouchers/:id` | Public | path `id` | 200 Voucher |
| `GET /api/shops/:shopId/vouchers` | Public | path `shopId` | 200 `[Voucher...]` |
| `POST /api/vouchers` | Admin | body `{"shop_id","title","sub_title","rules","pay_value","actual_value"}` | 201 Voucher |
| `POST /api/vouchers/seckill` | Admin | same fields plus `"stock"`, `"begin_time"`, `"end_time"` | 201 Voucher |

- `stock`, `begin_time` and `end_time` only appear on seckill vouchers.
- Times are RFC3339, for example `"2026-10-13T10:00:00Z"`.
- Money values are integers in the smallest unit.

#### Order (seckill & pay)

| Method & Path                         | Access   | Input                   | Success response                                                        |
| ------------------------------------- | -------- | ----------------------- | ----------------------------------------------------------------------- |
| `POST /api/voucher-order/seckill/:id` | Customer | path `id` = voucher id  | 202 `{"order_id":n,"status":"pending"}`                                 |
| `POST /api/voucher-order/:id/pay`     | Customer | path `id` = order id    | 200 Order                                                               |
| `GET /api/voucher-order/:id`          | Customer | path `id`               | 200 Order. Returns 404 while still pending or if the order isn't yours. |
| `GET /api/voucher-orders`             | Customer | query `limit`, `offset` | 200 `[Order...]`                                                        |

- `pay_time`, `use_time` and `refund_time` are left out until they apply.

**Seckill (S1 saga):** 202 means you are admitted and the order is created asynchronously (Kafka, then the S1 saga).

| Status | Reason |
|---|---|
| 409 | out of stock, or you already ordered this voucher |
| 425 | the sale hasn't started |
| 410 | the sale has ended |
| 404 | unknown voucher |
| 403 | admin cannot grab |

**Pay (S2 saga):** the request waits until the saga finishes.

| Status | Reason |
|---|---|
| 402 | rolled back, usually because the balance was too low. Nothing was charged, so you can retry after topping up. |
| 409 | the order isn't payable, because it isn't in `unpaid` status |


## Port-forwards that Tilt sets up to access resources

| What                  | Address                         |
| --------------------- | ------------------------------- |
| Postgres              | `localhost:5432` (`make psql`)  |
| Redis                 | `localhost:6379` (`make redis`) |
| Kafka                 | `localhost:9092`                |
| DTM console           | http://localhost:36789          |
| API (through ingress) | `http://localhost/api/...`      |
| Frontend (vite)       | http://localhost:5173 (`make frontend`) |


## Redis key

| Pattern                                          | Created by                   | When it appears                                                    |
| ------------------------------------------------ | ---------------------------- | ------------------------------------------------------------------ |
| `rl:ip:*`, `rl:user:*`, `rl:sms:*`, `rl:login:*` | gateway rate limiter         | any API call (each expires a few seconds after its bucket refills) |
| `login:code:{phone}`                             | gateway                      | after `/user/code` (2-minute TTL)                                  |
| `login:token:{token}`                            | gateway                      | after login (30-minute TTL)                                        |
| `cache:shop:{id}`, `cache:shop-types`            | shop-service                 | after `GET /api/shops/:id` or `/shop-types`                        |
| `follows:{userId}`                               | user-service                 | after a follow (no TTL)                                            |
| `seckill:stock:{vid}`                            | voucher-service              | on startup (`WarmStock`) or when a seckill voucher is created      |
| `seckill:order:{vid}`                            | voucher-service              | after someone is admitted                                          |
| `icr:order:YYYY:MM:DD`                           | voucher-service id generator | after an admitted seckill                                          |
