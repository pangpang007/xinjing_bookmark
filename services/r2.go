package services

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/soupcircle/bookjie-api/config"
)

type R2 struct {
	client    *s3.Client
	bucket    string
	publicURL string
	prefix    string
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
		client:    client,
		bucket:    cfg.R2BucketName,
		publicURL: cfg.R2PublicURL,
		prefix:    cfg.R2KeyPrefix,
	}, nil
}

func (r *R2) UploadJPEG(ctx context.Context, key string, data []byte) (string, error) {
	fullKey := r.objectKey(key)
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(fullKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		return "", err
	}
	return r.publicURL + "/" + fullKey, nil
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
