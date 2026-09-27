-- 000001_init.down.sql
-- Rollback initial schema

DROP TABLE IF EXISTS route_rules;
DROP TABLE IF EXISTS env_vars;
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS databases;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS user_quotas;
DROP TABLE IF EXISTS users;
