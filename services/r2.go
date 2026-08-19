package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/soupcircle/bookjie-api/config"
)

var shareImageNameRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.jpe?g$`)

type R2 struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewR2(cfg *config.Config) (*R2, error) {
	if cfg.R2AccountID == "" || cfg.R2AccessKey == "" || cfg.R2SecretKey == "" || cfg.R2BucketName == "" {
		return nil, fmt.Errorf("r2 is not fully configured")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID)
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			HostnameImmutable: true,
			Source:            aws.EndpointSourceCustom,
		}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2AccessKey, cfg.R2SecretKey, "")),
		awsconfig.WithRegion("auto"),
		awsconfig.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &R2{
		client: client,
		bucket: cfg.R2BucketName,
		prefix: cfg.R2KeyPrefix,
	}, nil
}

func (r *R2) UploadJPEG(ctx context.Context, key string, data []byte) (string, error) {
	return r.Put(ctx, key, "image/jpeg", data)
}

func (r *R2) Put(ctx context.Context, key, contentType string, data []byte) (string, error) {
	fullKey := r.objectKey(key)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(fullKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return fullKey, nil
}

func (r *R2) Exists(ctx context.Context, key string) (bool, error) {
	_, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.objectKey(key)),
	})
	if err == nil {
		return true, nil
	}
	if isS3NotFound(err) {
		return false, nil
	}
	return false, err
}

func (r *R2) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.objectKey(key)),
	})
	if err != nil {
		return nil, "", 0, err
	}
	contentType := "application/octet-stream"
	if out.ContentType != nil && *out.ContentType != "" {
		contentType = *out.ContentType
	}
	size := int64(-1)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, contentType, size, nil
}

func (r *R2) GetJPEG(ctx context.Context, filename string) (io.ReadCloser, string, int64, error) {
	name := ShareImageFilename(filename)
	if name == "" {
		return nil, "", 0, fmt.Errorf("invalid share image name")
	}
	body, contentType, size, err := r.Get(ctx, name)
	if err != nil {
		return nil, "", 0, err
	}
	if contentType == "application/octet-stream" {
		contentType = "image/jpeg"
	}
	return body, contentType, size, nil
}

func WxaCodeObjectKey(env string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = "release"
	}
	return "wxacode/share-" + env + ".png"
}

func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NotFoundException":
			return true
		}
	}
	return false
}

// ShareImageFilename extracts a UUID jpeg name from an object key or any stored URL.
func ShareImageFilename(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return ""
	}
	p := stored
	if u, err := url.Parse(stored); err == nil && u.Scheme != "" {
		p = u.Path
	}
	name := path.Base(p)
	if !shareImageNameRe.MatchString(name) {
		return ""
	}
	return strings.ToLower(name)
}

func (r *R2) objectKey(key string) string {
	key = strings.TrimLeft(key, "/")
	if r.prefix == "" {
		return key
	}
	if strings.HasPrefix(key, r.prefix+"/") {
		return key
	}
	return r.prefix + "/" + key
}
