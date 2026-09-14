package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// UploadMedia handles file upload to R2 storage
func UploadMedia(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userId := c.GetInt(string(constant.ContextKeyUserId))
	if userId == 0 {
		common.ApiErrorMsg(c, "unauthorized")
		return
	}

	// Check if R2 is enabled
	cfg := service.GetR2ConfigFromOptions()
	if !cfg.Enabled {
		common.ApiErrorMsg(c, "R2 storage is not enabled")
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "file is required")
		return
	}
	defer file.Close()

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		common.ApiError(c, fmt.Errorf("failed to read file: %w", err))
		return
	}

	// Detect content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Determine file type
	fileType := "file"
	if strings.HasPrefix(contentType, "image/") {
		fileType = "image"
	} else if strings.HasPrefix(contentType, "audio/") {
		fileType = "audio"
	} else if strings.HasPrefix(contentType, "video/") {
		fileType = "video"
	}

	// Upload to R2
	downloadURL, err := service.UploadToR2(c.Request.Context(), header.Filename, data, contentType)
	if err != nil {
		common.ApiError(c, fmt.Errorf("failed to upload to R2: %w", err))
		return
	}

	// Return response in the format specified by user
	common.ApiSuccess(c, gin.H{
		"type":         fileType,
		"download_url": downloadURL,
		"size":         fmt.Sprintf("%d", len(data)),
	})
}

// TestR2Connection tests R2 connection with provided or existing configuration
func TestR2Connection(c *gin.Context) {
	var req struct {
		Endpoint  string `json:"endpoint"`
		Bucket    string `json:"bucket"`
		KeyID     string `json:"key_id"`
		Secret    string `json:"secret"`
		PublicURL string `json:"public_url"`
	}

	// Try to parse request body
	if err := c.ShouldBindJSON(&req); err != nil {
		// If no request body, use existing configuration
		cfg := service.GetR2ConfigFromOptions()
		if !cfg.Enabled {
			common.ApiErrorMsg(c, "R2 storage is not configured")
			return
		}
		req.Endpoint = cfg.Endpoint
		req.Bucket = cfg.Bucket
		req.KeyID = cfg.KeyID
		req.Secret = cfg.Secret
		req.PublicURL = cfg.PublicURL
	}

	// Validate required fields
	if req.Endpoint == "" || req.Bucket == "" || req.KeyID == "" || req.Secret == "" {
		common.ApiErrorMsg(c, "missing required R2 configuration fields")
		return
	}

	// Create config and test connection
	cfg := &service.R2StorageConfig{
		Endpoint:  req.Endpoint,
		Bucket:    req.Bucket,
		KeyID:     req.KeyID,
		Secret:    req.Secret,
		PublicURL: req.PublicURL,
		Enabled:   true,
	}

	err := service.TestR2Connection(cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"message": "R2 connection successful",
		"bucket":  req.Bucket,
	})
}

// UpdateR2Settings updates R2 storage configuration
func UpdateR2Settings(c *gin.Context) {
	var req struct {
		Endpoint    string `json:"endpoint"`
		Bucket      string `json:"bucket"`
		KeyID       string `json:"key_id"`
		Secret      string `json:"secret"`
		PublicURL   string `json:"public_url"`
		StoragePath string `json:"storage_path"`
		Enabled     bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	// Build update map for UpdateOptionsBulk
	updateMap := make(map[string]string)

	if req.Enabled {
		updateMap["R2StorageEnabled"] = "true"
	} else {
		updateMap["R2StorageEnabled"] = "false"
	}

	updateMap["R2Endpoint"] = req.Endpoint
	updateMap["R2Bucket"] = req.Bucket
	updateMap["R2PublicURL"] = strings.TrimSuffix(req.PublicURL, "/")
	updateMap["R2StoragePath"] = req.StoragePath

	// Only update KeyID and Secret if provided (not empty)
	if req.KeyID != "" {
		updateMap["R2KeyID"] = req.KeyID
	}
	if req.Secret != "" {
		updateMap["R2Secret"] = req.Secret
	}

	// Update options in bulk
	err := model.UpdateOptionsBulk(updateMap)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"message": "R2 storage settings updated successfully",
	})
}
