DROP DATABASE IF EXISTS go_twitter CASCADE;
CREATE DATABASE IF NOT EXISTS go_twitter;
SET DATABASE = go_twitter;
CREATE TABLE IF NOT EXISTS users (
    id SERIAL NOT NULL PRIMARY KEY,
    email VARCHAR NOT NULL UNIQUE,
    username VARCHAR NOT NULL UNIQUE,
    avatar VARCHAR,
    followers_count INT NOT NULL DEFAULT 0 CHECK (followers_count >= 0),
    followees_count INT NOT NULL DEFAULT 0 CHECK (followees_count >= 0)

)
CREATE TABLE IF NOT EXISTS follows (
  follower_id INT NOT NULL REFERENCES users,
  followee_id INT NOT NULL REFERENCES users,
  PRIMARY KEY (follower_id, followee_id)
)

CREATE TABLE IF NOT EXISTS posts (
   id SERIAL NOT NULL PRIMARY KEY,
   user_id INT NOT NULL REFERENCES users,
   content VARCHAR NOT NULL,
   spoiler_of VARCHAR,
   nsfw BOOLEAN NOT NULL,
   likes_count INT NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
   comments_count INT NOT NULL DEFAULT 0 CHECK (comments_count >= 0),
   created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)
CREATE INDEX IF NOT EXISTS sorted_posts ON posts(created_at DESC);

CREATE TABLE IF NOT EXISTS timeline (
   id SERIAL NOT NULL PRIMARY KEY,
   user_id INT NOT NULL REFERENCES users,
   post_id INT NOT NULL REFERENCES posts
)
CREATE UNIQUE INDEX IF NOT EXISTS timeline_unique ON timeline (user_id, post_id);

CREATE TABLE IF NOT EXISTS post_likes (
   user_id INT NOT NULL REFERENCES users,
   post_id INT NOT NULL REFERENCES posts,
   PRIMARY KEY (user_id, post_id)
)


CREATE TABLE IF NOT EXISTS post_subscriptions (
   user_id INT NOT NULL REFERENCES users,
   post_id INT NOT NULL REFERENCES posts,
   PRIMARY KEY (user_id, post_id)
)


CREATE TABLE IF NOT EXISTS comments (
   id SERIAL NOT NULL PRIMARY KEY,
   user_id INT NOT NULL REFERENCES users,
   post_id INT NOT NULL REFERENCES posts,
   content VARCHAR NOT NULL,
   likes_count INT NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
   created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

CREATE TABLE IF NOT EXISTS comment_likes (
   user_id INT NOT NULL REFERENCES users,
   comment_id INT NOT NULL REFERENCES comments,
   PRIMARY KEY (user_id, comment_id)
)

CREATE TABLE IF NOT EXISTS notifications (
     id SERIAL NOT NULL PRIMARY KEY,
     user_id INT NOT NULL REFERENCES users,
     post_id INT NOT NULL REFERENCES posts,
     actors VARCHAR[] NOT NULL,
     type VARCHAR NOT NULL,
     read BOOLEAN NOT NULL DEFAULT false,
     issued_at TIMESTAMPTZ NOT NULL DEFAULT now()
 );

CREATE INDEX IF NOT EXISTS sorted_notifications ON notifications (issued_at DESC);
CREATE INDEX IF NOT EXISTS unique_notifications ON notifications (user_id, type, post_id, read);

INSERT INTO users (id, email, username) VALUES
    (1, 'mohammedosama@ieee.org', 'mohammedosama'),
    (2, 'ahmedosama@ieee.org', 'ahmedosama');

INSERT INTO posts (id, user_id, content, nsfw, comments_count) VALUES
    (1, 1, 'sample post', false, 0);
INSERT INTO post_subscriptions (user_id, post_id) VALUES (1,1);
INSERT INTO timeline (id, user_id, post_id) VALUES
    (1, 1, 1);
INSERT INTO comments (id, user_id, post_id, content, likes_count) VALUES
    (1, 1, 1, 'sample-comment', 0);
