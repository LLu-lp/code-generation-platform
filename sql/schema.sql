-- Go 版本数据库初始化
-- 创建库
CREATE DATABASE IF NOT EXISTS yu_ai_code_mother DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE yu_ai_code_mother;

-- 用户表
CREATE TABLE IF NOT EXISTS user (
    id           BIGINT AUTO_INCREMENT COMMENT 'id' PRIMARY KEY,
    userAccount  VARCHAR(256) NOT NULL COMMENT '账号',
    userPassword VARCHAR(512) NOT NULL COMMENT '密码',
    userName     VARCHAR(256) NULL COMMENT '用户昵称',
    userAvatar   VARCHAR(1024) NULL COMMENT '用户头像',
    userProfile  VARCHAR(512) NULL COMMENT '用户简介',
    userRole     VARCHAR(256) DEFAULT 'user' NOT NULL COMMENT '用户角色：user/admin',
    editTime     DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL COMMENT '编辑时间',
    createTime   DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL COMMENT '创建时间',
    updateTime   DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    isDelete     TINYINT DEFAULT 0 NOT NULL COMMENT '是否删除',
    UNIQUE KEY uk_userAccount (userAccount),
    INDEX idx_userName (userName)
) COMMENT '用户' ENGINE=InnoDB;

-- 应用表
CREATE TABLE IF NOT EXISTS app (
    id           BIGINT AUTO_INCREMENT COMMENT 'id' PRIMARY KEY,
    appName      VARCHAR(256) NULL COMMENT '应用名称',
    cover        VARCHAR(512) NULL COMMENT '应用封面',
    initPrompt   TEXT NULL COMMENT '应用初始化的 prompt',
    codeGenType  VARCHAR(64) NULL COMMENT '代码生成类型（html/multi_file/vue_project）',
    deployKey    VARCHAR(64) NULL COMMENT '部署标识',
    deployedTime DATETIME NULL COMMENT '部署时间',
    priority     INT DEFAULT 0 NOT NULL COMMENT '优先级（99=精选）',
    userId       BIGINT NOT NULL COMMENT '创建用户id',
    editTime     DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL COMMENT '编辑时间',
    createTime   DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL COMMENT '创建时间',
    updateTime   DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    isDelete     TINYINT DEFAULT 0 NOT NULL COMMENT '是否删除',
    UNIQUE KEY uk_deployKey (deployKey),
    INDEX idx_appName (appName),
    INDEX idx_userId (userId)
) COMMENT '应用' ENGINE=InnoDB;

-- 对话历史表
CREATE TABLE IF NOT EXISTS chat_history (
    id          BIGINT AUTO_INCREMENT COMMENT 'id' PRIMARY KEY,
    message     TEXT NOT NULL COMMENT '消息',
    messageType VARCHAR(32) NOT NULL COMMENT 'user/ai',
    appId       BIGINT NOT NULL COMMENT '应用id',
    userId      BIGINT NOT NULL COMMENT '创建用户id',
    createTime  DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL COMMENT '创建时间',
    updateTime  DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    isDelete    TINYINT DEFAULT 0 NOT NULL COMMENT '是否删除',
    INDEX idx_appId (appId),
    INDEX idx_createTime (createTime),
    INDEX idx_appId_createTime (appId, createTime)
) COMMENT '对话历史' ENGINE=InnoDB;
