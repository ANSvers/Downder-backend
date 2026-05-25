package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	BaseDir string
}

func NewLocalStorage(baseDir string) *LocalStorage {
	_ = os.MkdirAll(baseDir, os.ModePerm)
	return &LocalStorage{
		BaseDir: baseDir,
	}
}

// SaveStream รับข้อมูล Stream มาเขียนลงไฟล์
func (l *LocalStorage) SaveStream(filename string, stream io.Reader) (string, error) {
	safeFilename := filepath.Base(filename)
	fullPath := filepath.Join(l.BaseDir, safeFilename)

	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", safeFilename, err)
	}
	defer out.Close()

	_, err = io.Copy(out, stream)
	if err != nil {
		return "", fmt.Errorf("failed to save data: %w", err)
	}

	return fullPath, nil
}

func (l *LocalStorage) DeleteFile(filename string) error {
	safeFilename := filepath.Base(filename)
	return os.Remove(filepath.Join(l.BaseDir, safeFilename))
}
