package model

import "time"

// FileQuery represents filters for searching files.
type FileQuery struct {
	Category  *Category  `json:"category,omitempty"`
	Tag       *string    `json:"tag,omitempty"`
	Search    *string    `json:"search,omitempty"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`
	Limit     int        `json:"limit"`
	NextToken *string    `json:"nextToken,omitempty"`
}

// FileListResult holds a paginated list of files.
type FileListResult struct {
	Files     []File  `json:"files"`
	NextToken *string `json:"nextToken,omitempty"`
}

// CategoryCount holds a category and its file count.
type CategoryCount struct {
	Category Category `json:"category"`
	Count    int      `json:"count"`
}
