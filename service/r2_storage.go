package service

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2StorageConfig holds Cloudflare R2 configuration
type R2StorageConfig struct {
	Endpoint    string
	Bucket      string
	KeyID       string
	Secret      string
	PublicURL   string
	Enabled     bool
	StoragePath string // Optional prefix path in bucket, e.g., "uploads/"
}

// R2StorageService handles file uploads to Cloudflare R2
type R2StorageService struct {
	config *R2StorageConfig
	client *s3.Client
}

// NewR2StorageService creates a new R2 storage service
func NewR2StorageService(cfg *R2StorageConfig) (*R2StorageService, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("R2 storage is not enabled")
	}

	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.KeyID == "" || cfg.Secret == "" {
		return nil, fmt.Errorf("R2 storage configuration is incomplete")
	}

	// Create AWS SDK configuration for R2
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: cfg.Endpoint,
		}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.KeyID,
			cfg.Secret,
			"",
		)),
		config.WithRegion("auto"), // R2 uses "auto" region
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &R2StorageService{
		config: cfg,
		client: client,
	}, nil
}

// UploadFile uploads a file to R2 and returns the public URL
func (s *R2StorageService) UploadFile(ctx context.Context, filename string, data []byte, contentType string) (string, error) {
	// Generate unique filename with timestamp to avoid collisions
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	timestamp := time.Now().Format("20060102150405")
	uniqueFilename := fmt.Sprintf("%s_%s%s", name, timestamp, ext)

	// Add storage path prefix if configured
	key := uniqueFilename
	if s.config.StoragePath != "" {
		key = filepath.ToSlash(filepath.Join(s.config.StoragePath, uniqueFilename))
	}

	// Upload to R2
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to R2: %w", err)
	}

	// Build public URL
	publicURL := s.buildPublicURL(key)

	return publicURL, nil
}

// buildPublicURL constructs the public URL for an uploaded file
func (s *R2StorageService) buildPublicURL(key string) string {
	// Remove trailing slash from public URL base
	baseURL := strings.TrimSuffix(s.config.PublicURL, "/")
	// Ensure key starts without slash
	key = strings.TrimPrefix(key, "/")
	return fmt.Sprintf("%s/%s", baseURL, key)
}

// GetR2ConfigFromOptions retrieves R2 configuration from global options
func GetR2ConfigFromOptions() *R2StorageConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	enabled := common.OptionMap["R2StorageEnabled"] == "true"
	endpoint := common.OptionMap["R2Endpoint"]
	bucket := common.OptionMap["R2Bucket"]
	keyID := common.OptionMap["R2KeyID"]
	secret := common.OptionMap["R2Secret"]
	publicURL := common.OptionMap["R2PublicURL"]
	storagePath := common.OptionMap["R2StoragePath"]

	return &R2StorageConfig{
		Endpoint:    endpoint,
		Bucket:      bucket,
		KeyID:       keyID,
		Secret:      secret,
		PublicURL:   publicURL,
		Enabled:     enabled,
		StoragePath: storagePath,
	}
}

// TestR2Connection tests the R2 connection by attempting to list buckets
func TestR2Connection(cfg *R2StorageConfig) error {
	service, err := NewR2StorageService(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test by attempting to head the bucket
	_, err = service.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access R2 bucket: %w", err)
	}

	return nil
}

// UploadToR2 is a convenience function that uploads a file using the global R2 configuration
func UploadToR2(ctx context.Context, filename string, data []byte, contentType string) (string, error) {
	cfg := GetR2ConfigFromOptions()
	if !cfg.Enabled {
		return "", fmt.Errorf("R2 storage is not enabled")
	}

	service, err := NewR2StorageService(cfg)
	if err != nil {
		return "", err
	}

	return service.UploadFile(ctx, filename, data, contentType)
}
