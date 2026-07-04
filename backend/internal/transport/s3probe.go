package transport

// s3EventProbe sniffs just enough of a raw event to detect an S3 notification.
type s3EventProbe struct {
	Records []s3EventProbeRecord `json:"Records"`
}
