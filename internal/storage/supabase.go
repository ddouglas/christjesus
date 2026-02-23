package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// SupabaseStorage handles file uploads to Supabase Storage
type SupabaseStorage struct {
	projectID  string
	apiKey     string
	bucketName string
	httpClient *http.Client
}

// NewSupabaseStorage creates a new Supabase Storage client
func NewSupabaseStorage(projectID, apiKey, bucketName string) *SupabaseStorage {
	return &SupabaseStorage{
		projectID:  projectID,
		apiKey:     apiKey,
		bucketName: bucketName,
		httpClient: &http.Client{},
	}
}

// UploadFile uploads a file to Supabase Storage
// Returns the storage key (path) on success
func (s *SupabaseStorage) UploadFile(ctx context.Context, path string, file multipart.File, contentType string) (string, error) {
	url := fmt.Sprintf("https://%s.supabase.co/storage/v1/object/%s/%s",
		s.projectID, s.bucketName, path)

	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(fileBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Content-Type", contentType)

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return path, nil
}

// DeleteFile removes a file from Supabase Storage
func (s *SupabaseStorage) DeleteFile(ctx context.Context, path string) error {
	url := fmt.Sprintf("https://%s.supabase.co/storage/v1/object/%s/%s",
		s.projectID, s.bucketName, path)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetPublicURL returns the public URL for a file
func (s *SupabaseStorage) GetPublicURL(path string) string {
	return fmt.Sprintf("https://%s.supabase.co/storage/v1/object/public/%s/%s",
		s.projectID, s.bucketName, path)
}
