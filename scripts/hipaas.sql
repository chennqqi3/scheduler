USE hipaas;
 
SET NAMES utf8;
 
/**
* app name要作为域名的一部分，所以全局唯一
* app有个创建者creator
* app一般有多个人来维护，所以对应一个team
* memory的单位是MB
* instance是实例数目
* status: 表示app的状态，running、flapping之类的
*/
DROP TABLE IF EXISTS app;
CREATE TABLE app (
  id       INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name     VARCHAR(64)  NOT NULL UNIQUE,
  creator  INT UNSIGNED NOT NULL,
  team_id  INT UNSIGNED NOT NULL DEFAULT 0,
  team_name varchar(64) NOT NULL DEFAULT '',
  region   varchar(64)  NOT NULL DEFAULT '',
  image    VARCHAR(255) NOT NULL DEFAULT '',
  mount    VARCHAR(8192) NOT NULL DEFAULT '',
  recovery BOOLEAN      NOT NULL DEFAULT TRUE,
  vmtype   varchar(64)  NOT NULL DEFAULT '',
  memory   INT          NOT NULL DEFAULT 4096,
  cpu      INT          NOT NULL DEFAULT 2,
  health   varchar(128) NOT NULL DEFAULT '',
  instance INT          NOT NULL DEFAULT 0
  COMMENT 'instance count',
  status   TINYINT      NOT NULL DEFAULT 0,
  KEY idx_app_team(team_id)
)
ENGINE =innodb
  DEFAULT CHARSET =utf8
  COLLATE =utf8_general_ci;
 
DROP TABLE IF EXISTS domain;
CREATE TABLE domain (
  id           INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  domain       VARCHAR(255) NOT NULL UNIQUE,
  team_id      INT UNSIGNED NOT NULL DEFAULT 0,
  team_name    VARCHAR(64) NOT NULL  DEFAULT '',
  app_id       INT UNSIGNED NOT NULL DEFAULT 0,
  app_name     VARCHAR(64)  NOT NULL DEFAULT '',
  creator      INT UNSIGNED NOT NULL,
  creator_name VARCHAR(64)  NOT NULL,
  bind_user_id VARCHAR(64)  NOT NULL,
  bind_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_domain_app_id(app_id)
)
  ENGINE =innodb
  DEFAULT CHARSET =utf8
  COLLATE =utf8_general_ci;
/*
* 对于某个app，可以设置一些环境变量，比如DB连接地址，先期可以手填，之后可以结合rds使用
*/
DROP TABLE IF EXISTS env;
CREATE TABLE env (
  id       INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  k        VARCHAR(128) NOT NULL,
  v        VARCHAR(1024) NOT NULL,
  app_id   INT UNSIGNED NOT NULL,
  app_name VARCHAR(64)  NOT NULL,
  KEY idx_env_app_id(app_id)
)
  ENGINE =innodb
  DEFAULT CHARSET =utf8
  COLLATE =utf8_general_ci;
 
/*
* 有多少个APP实例，就应该有对应的Hostname个数。
*/
DROP TABLE IF EXISTS hostname;
CREATE TABLE hostname (
  id           INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
  hostname     VARCHAR(255)  NOT NULL,
  subdomain    VARCHAR(255)  NOT NULL,
  log          VARCHAR(1024) NOT NULL,
  status       TINYINT       NOT NULL DEFAULT 0,
  app_id       INT UNSIGNED  NOT NULL,
  app_name     VARCHAR(64)   NOT NULL,
  KEY idx_hostname_app_id(app_id)
)
  ENGINE =innodb
  DEFAULT CHARSET =utf8
  COLLATE =utf8_general_ci;
 
DROP TABLE IF EXISTS history;
CREATE TABLE history (
  id        INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
  app_id    INT UNSIGNED  NOT NULL,
  app_name  VARCHAR(64)   NOT NULL,
  resume    VARCHAR(255)  NOT NULL DEFAULT '',
  image     VARCHAR(1024) NOT NULL DEFAULT '',
  create_at TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_history_app_id(app_id)
)
ENGINE =innodb
  DEFAULT CHARSET =utf8
  COLLATE =utf8_general_ci; 
 
DROP TABLE IF EXISTS image;
CREATE TABLE image (
  id                   INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
  app_id    INT UNSIGNED  NOT NULL,
  docker_image_url     VARCHAR(255)  NOT NULL,
  docker_login_server  VARCHAR(255)  DEFAULT '',
  docker_user          VARCHAR(255)  DEFAULT '', 
  docker_password      VARCHAR(255)  DEFAULT '',
  docker_email         VARCHAR(255)  DEFAULT '',
  KEY idx_image_app_id(app_id)
)
  ENGINE =innodb
  DEFAULT CHARSET =utf8
  COLLATE =utf8_general_ci;
