package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type supabaseStorage struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

func NewSupabaseStorage() (Storage, error) {
	endpoint := os.Getenv("SUPABASE_S3_ENDPOINT")
	accessKey := os.Getenv("SUPABASE_S3_ACCESS_KEY")
	secretKey := os.Getenv("SUPABASE_S3_SECRET_KEY")
	bucket := os.Getenv("SUPABASE_S3_BUCKET")
	publicBase := os.Getenv("SUPABASE_STORAGE_PUBLIC_URL")

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" || publicBase == "" {
		return nil, fmt.Errorf("supabase storage is not configured")
	}

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		UsePathStyle: true,
	})

	return &supabaseStorage{
		client:        client,
		bucket:        bucket,
		publicBaseURL: strings.TrimRight(publicBase, "/"),
	}, nil
}

func (s *supabaseStorage) Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return s.publicBaseURL + "/" + key, nil
}

func (s *supabaseStorage) DeleteObjectsByPrefix(ctx context.Context, prefix string) error {
	var keys []string
	var continuationToken *string

	for {
		listOut, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return err
		}

		for _, obj := range listOut.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}

		if listOut.IsTruncated == nil || !*listOut.IsTruncated {
			break
		}
		continuationToken = listOut.NextContinuationToken
	}

	if len(keys) == 0 {
		return nil
	}

	for start := 0; start < len(keys); start += 1000 {
		end := start + 1000
		if end > len(keys) {
			end = len(keys)
		}

		objects := make([]types.ObjectIdentifier, 0, end-start)
		for _, key := range keys[start:end] {
			objects = append(objects, types.ObjectIdentifier{Key: aws.String(key)})
		}

		_, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: objects},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
