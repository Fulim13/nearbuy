INSERT INTO shop_type (id, name, icon, sort_order) VALUES
    (1,  'Food',        '/types/ms.png',    1),
    (2,  'KTV',         '/types/KTV.png',   2),
    (3,  'Hair Salon',  '/types/lrmf.png',  3),
    (4,  'Gym',         '/types/jsyd.png', 10),
    (5,  'Massage',     '/types/amzl.png',  5),
    (6,  'Beauty SPA',  '/types/spa.png',   6),
    (7,  'Kids Play',   '/types/qzyl.png',  7),
    (8,  'Bar',         '/types/jiuba.png', 8),
    (9,  'Party House', '/types/hpg.png',   9),
    (10, 'Nails',       '/types/mjmj.png',  4);

INSERT INTO shop (id, name, type_id, images, area, address, x, y, avg_price, sold, comments, score, open_hours) VALUES
    (1,  '103 Tea Restaurant',   1, '/imgs/shops/1.jpg',  'Daguan',       '29 Jinhua Rd',        120.149192, 30.316078,  80,  4215, 3035, 37, '10:00-22:00'),
    (2,  'Caima Grill Hotpot',   1, '/imgs/shops/2.jpg',  'Gongchenqiao', '1035 Shangtang Rd',   120.151505, 30.333422,  85,  2160, 1460, 46, '11:30-03:00'),
    (3,  'Xin Bai Lu',           1, '/imgs/shops/3.jpg',  'Yunhe St',     '2 Taizhou Rd F5',     120.151954, 30.324970,  61, 12035, 8045, 47, '10:30-21:00'),
    (4,  'Mamala',               1, '/imgs/shops/4.jpg',  'Gongchenqiao', '66 Lishui Rd B115',   120.146659, 30.312742, 290, 13519, 9529, 49, '11:00-22:00'),
    (5,  'Haidilao Hotpot',      1, '/imgs/shops/5.jpg',  'Daguan',       '458 Shangtang Rd F6', 120.157780, 30.310633, 104,  4125, 2764, 49, '10:00-07:00'),
    (6,  'Xingfuli Hotpot',      1, '/imgs/shops/6.jpg',  'Gongchenqiao', '189 Jinhua South Rd', 120.148603, 30.318618, 130,  9531, 7324, 46, '11:00-13:50'),
    (7,  'Luyu Grilled Fish',    1, '/imgs/shops/7.jpg',  'North City',   '666 Hangxing Rd',     120.124691, 30.336819,  85,  2631, 1320, 47, '00:00-24:00'),
    (8,  'Qiancaowu Sushi',      1, '/imgs/shops/8.jpg',  'Yunhe St',     '80 Jinhua Rd B1',     120.150526, 30.325231,  88,  2406, 1206, 46, '11:00-21:30'),
    (9,  'Yanglaosan Hotpot',    1, '/imgs/shops/9.jpg',  'Yunhe St',     '2 Taizhou Rd F5',     120.150598, 30.325251, 101,  2763, 1363, 44, '11:00-21:30'),
    (10, 'Kaledi KTV',           2, '/imgs/shops/10.jpg', 'Yunhe St',     '2 Taizhou Rd F4',     120.149093, 30.324666,  67, 26891,  902, 37, '00:00-24:00'),
    (11, 'INLOVE KTV',           2, '/imgs/shops/11.jpg', 'Crystal City', '458 Shangtang Rd 6F', 120.158530, 30.310002,  75, 35977, 5684, 47, '11:30-06:00'),
    (12, 'Mei KTV',              2, '/imgs/shops/12.jpg', 'Ocean Park',   '58 Lishui Rd F4',     120.149830, 30.312110,  88,  6444,  235, 46, '10:00-02:00'),
    (13, 'OuK La KTV',           2, '/imgs/shops/13.jpg', 'D32 Tianyang', '567 Huzhou St 5F',    120.130453, 30.327655,  58, 18997, 1857, 41, '12:00-02:00'),
    (14, 'Xingjuhui KTV',        2, '/imgs/shops/14.jpg', 'North City',   '666 Hangxing Rd C',   120.128958, 30.337252,  60, 17771,  685, 47, '10:00-22:00');

-- Seeding with explicit ids does NOT advance the identity sequence. Without this,
-- the next INSERT without an id collides on the primary key.
SELECT setval(pg_get_serial_sequence('shop_type', 'id'), (SELECT max(id) FROM shop_type));
SELECT setval(pg_get_serial_sequence('shop',      'id'), (SELECT max(id) FROM shop));
