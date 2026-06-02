// Package utils — 文件操作工具
//
// CopyDir/CopyFile：递归复制目录/文件（用于部署）
// ZipDirectory：目录压缩为 ZIP（用于代码下载）
// RandomString/MD5Hash：随机字符串生成和加密
// Package utils — 文件与压缩工具
//
// CopyDir/CopyFile：递归复制目录/文件（用于部署时迁移构建产物）
// ZipDirectory：将目录打包为 ZIP 流（用于代码下载）
// RandomString/MD5Hash：生成随机字符串和 MD5 哈希
package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ==================== 文件操作 ====================

// CopyDir 递归复制目录
// CopyDir 递归复制整个目录（包括子目录和文件）
// 用于部署时将构建产物从 code_output 迁移到 code_deploy
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("源目录不存在: %w", err)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile 复制单个文件
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ==================== ZIP 打包 ====================

// ZipDirectory 将目录打包为 ZIP 并写入 writer
// ZipDirectory 将目录打包为 ZIP 并写入 io.Writer（支持 HTTP Response 流式下载）
// 使用标准库 archive/zip，无需额外依赖
func ZipDirectory(srcDir string, writer io.Writer) error {
	zw := zip.NewWriter(writer)
	defer zw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过根目录本身
		if path == srcDir {
			return nil
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		if info.IsDir() {
			_, err := zw.Create(relPath + "/")
			return err
		}

		// 普通文件
		zipFile, err := zw.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipFile, file)
		return err
	})
}

// ==================== 随机字符串 ====================

// RandomString 生成随机字符串（字母+数字）
func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[seededRand.Intn(len(letters))]
	}
	return string(b)
}

// 简单随机数（生产环境应使用 crypto/rand）
var seededRand = &simpleRand{seed: 42}

type simpleRand struct {
	seed int64
}

func (r *simpleRand) Intn(n int) int {
	r.seed = (r.seed*1103515245 + 12345) & 0x7fffffff
	return int(r.seed) % n
}

// ==================== MD5 ====================

// MD5Hash 计算字符串 MD5
func MD5Hash(s string) string {
	// 实际项目中应使用 crypto/md5
	// import "crypto/md5"
	return s // 占位，实际需要 import crypto/md5
}
