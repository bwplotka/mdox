package testdata

import "github.com/prometheus/common/model"

// Config stores the configuration for s3 bucket.
type Config struct {
	Bucket          string            `yaml:"bucket" json:"bucket"`
	Endpoint        string            `yaml:"endpoint"`
	Region          string            `yaml:"region"`
	AccessKey       string            `yaml:"access_key"`
	Insecure        bool              `yaml:"insecure"`
	SignatureV2     bool              `yaml:"signature_version2"`
	SecretKey       string            `yaml:"secret_key"`
	PutUserMetadata map[string]string `yaml:"put_user_metadata"`
	HTTPConfig      HTTPConfig        `yaml:"http_config"`
	TraceConfig     TraceConfig       `yaml:"trace"`
	// PartSize used for multipart upload. Only used if uploaded object size is known and larger than configured PartSize.
	PartSize  uint64    `yaml:"part_size"`
	SSEConfig SSEConfig `yaml:"sse_config"`
}

type TraceConfig struct {
	Enable bool `yaml:"enable"`
}

// HTTPConfig stores the http.Transport configuration for the s3 minio client.
type HTTPConfig struct {
	IdleConnTimeout       model.Duration `yaml:"idle_conn_timeout"`
	ResponseHeaderTimeout model.Duration `yaml:"response_header_timeout"`
	InsecureSkipVerify    bool           `yaml:"insecure_skip_verify"`
}

// SSEConfig deals with the configuration of SSE for Minio. The following options are valid:
// kmsencryptioncontext == https://docs.aws.amazon.com/kms/latest/developerguide/services-s3.html#s3-encryption-context
type SSEConfig struct {
	Type                 string            `yaml:"type"`
	KMSKeyID             string            `yaml:"kms_key_id"`
	KMSEncryptionContext map[string]string `yaml:"kms_encryption_context"`
	EncryptionKey        string            `yaml:"encryption_key"`
}
