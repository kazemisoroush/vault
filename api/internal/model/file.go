package model

import "time"

// File represents a file's metadata stored in the vault.
type File struct {
	ID           string    `json:"id" dynamodbav:"id"`
	DriveFileID  string    `json:"driveFileId" dynamodbav:"driveFileId"`
	Name         string    `json:"name" dynamodbav:"name"`
	MimeType     string    `json:"mimeType" dynamodbav:"mimeType"`
	Size         int64     `json:"size" dynamodbav:"size"`
	Category     Category  `json:"category" dynamodbav:"category"`
	Tags         []string  `json:"tags" dynamodbav:"tags"`
	DrivePath    string    `json:"drivePath" dynamodbav:"drivePath"`
	ThumbnailURL string    `json:"thumbnailUrl,omitempty" dynamodbav:"thumbnailUrl,omitempty"`
	WebViewURL   string    `json:"webViewUrl,omitempty" dynamodbav:"webViewUrl,omitempty"`
	CreatedAt    time.Time `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt" dynamodbav:"updatedAt"`
}
