/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backuputil

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// S3Params is everything needed to reach a bucket.
type S3Params struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

// ReadS3Credentials reads the access key pair from a Secret through the
// uncached clientset. Reading it through the manager's client would start a
// cluster-wide Secret informer in the operator — every Secret in every
// namespace held in its memory — which is why the rest of the operator also
// reads Secrets this way.
func ReadS3Credentials(ctx context.Context, cs kubernetes.Interface, namespace, name string) (accessKey, secretKey string, err error) {
	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", "", fmt.Errorf("secret %q not found in namespace %q", name, namespace)
		}
		return "", "", fmt.Errorf("failed to look up secret %q: %w", name, err)
	}
	for _, key := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		if _, ok := secret.Data[key]; !ok {
			return "", "", fmt.Errorf("secret %q is missing required key %q", name, key)
		}
	}
	return string(secret.Data["AWS_ACCESS_KEY_ID"]), string(secret.Data["AWS_SECRET_ACCESS_KEY"]), nil
}

// NewS3Client builds an S3 client, using path-style addressing whenever a
// custom endpoint is configured so S3-compatible servers such as MinIO work.
func NewS3Client(ctx context.Context, p S3Params) (*s3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(p.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(p.AccessKey, p.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if p.Endpoint != "" {
			o.BaseEndpoint = &p.Endpoint
			o.UsePathStyle = true
		}
	}), nil
}

// ParseS3URI splits "s3://bucket/some/prefix" into its parts. A location with
// no scheme is treated as a bare prefix in the caller's configured bucket.
func ParseS3URI(location, defaultBucket string) (bucket, prefix string, err error) {
	trimmed := strings.TrimSpace(location)
	if trimmed == "" {
		return "", "", fmt.Errorf("backup location must not be empty")
	}
	if rest, ok := strings.CutPrefix(trimmed, "s3://"); ok {
		b, p, found := strings.Cut(rest, "/")
		if !found || b == "" || p == "" {
			return "", "", fmt.Errorf("malformed backup location %q; expected s3://<bucket>/<prefix>", location)
		}
		return b, strings.Trim(p, "/"), nil
	}
	if defaultBucket == "" {
		return "", "", fmt.Errorf("backup location %q has no bucket and spec.s3.bucket is empty", location)
	}
	return defaultBucket, strings.Trim(trimmed, "/"), nil
}

// UploadDir uploads every regular file under localDir, preserving the relative
// directory structure beneath prefix.
//
// The transfer manager switches to multipart above its part size, so a single
// RDB or AOF larger than the 5 GiB PutObject limit is not an EntityTooLarge
// failure discovered only after the whole snapshot was taken.
func UploadDir(ctx context.Context, c *s3.Client, bucket, prefix, localDir string) error {
	uploader := transfermanager.New(c)
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		// #nosec G304,G122 -- path is produced by walking localDir, a directory
		// this process created under os.MkdirTemp.
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer f.Close()

		key := prefix + "/" + filepath.ToSlash(rel)
		if _, err := uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
			Bucket: &bucket,
			Key:    &key,
			Body:   f,
		}); err != nil {
			return fmt.Errorf("failed to upload s3://%s/%s: %w", bucket, key, err)
		}
		return nil
	})
}

// DownloadPrefix pulls every object under prefix into localDir, recreating the
// relative layout. It returns how many objects were written.
func DownloadPrefix(ctx context.Context, c *s3.Client, bucket, prefix, localDir string) (int, error) {
	p := strings.TrimSuffix(prefix, "/") + "/"
	pager := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &p,
	})

	// Object keys are attacker-influenced input, so every write is scoped to
	// an *os.Root: the kernel refuses any path resolving outside localDir.
	root, err := os.OpenRoot(localDir)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s as a root: %w", localDir, err)
	}
	defer root.Close()

	count := 0
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return count, fmt.Errorf("failed to list s3://%s/%s: %w", bucket, p, err)
		}
		for _, obj := range page.Contents {
			key := *obj.Key
			if strings.HasSuffix(key, "/") {
				continue
			}
			rel, err := sanitizeEntryName(strings.TrimPrefix(key, p))
			if err != nil {
				return count, err
			}
			if rel == "" {
				continue
			}
			if parent := filepath.Dir(rel); parent != "." {
				if err := root.MkdirAll(parent, 0o750); err != nil {
					return count, fmt.Errorf("failed to create %s: %w", parent, err)
				}
			}
			if err := downloadObject(ctx, c, root, bucket, key, rel); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

func downloadObject(ctx context.Context, c *s3.Client, root *os.Root, bucket, key, dest string) error {
	resp, err := c.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return fmt.Errorf("failed to download s3://%s/%s: %w", bucket, key, err)
	}
	defer resp.Body.Close()

	out, err := root.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dest, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}
	return out.Close()
}

// ListPrefixes returns the immediate "sub-directories" under prefix, which the
// backup layout uses one-per-run for retention decisions.
func ListPrefixes(ctx context.Context, c *s3.Client, bucket, prefix string) ([]string, error) {
	p := strings.TrimSuffix(prefix, "/") + "/"
	delim := "/"
	pager := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    &p,
		Delimiter: &delim,
	})

	var out []string
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list s3://%s/%s: %w", bucket, p, err)
		}
		for _, cp := range page.CommonPrefixes {
			if cp.Prefix != nil {
				out = append(out, strings.TrimSuffix(*cp.Prefix, "/"))
			}
		}
	}
	return out, nil
}

// DeletePrefix removes every object beneath prefix. It is used both by the
// retention sweep and by the finalizer, so deleting a RedisBackup no longer
// leaves its objects behind forever.
func DeletePrefix(ctx context.Context, c *s3.Client, bucket, prefix string) (int, error) {
	p := strings.TrimSuffix(prefix, "/") + "/"
	pager := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &p})

	deleted := 0
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return deleted, fmt.Errorf("failed to list s3://%s/%s: %w", bucket, p, err)
		}
		if len(page.Contents) == 0 {
			continue
		}
		ids := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			ids = append(ids, types.ObjectIdentifier{Key: obj.Key})
		}
		quiet := true
		if _, err := c.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &bucket,
			Delete: &types.Delete{Objects: ids, Quiet: &quiet},
		}); err != nil {
			return deleted, fmt.Errorf("failed to delete objects under s3://%s/%s: %w", bucket, p, err)
		}
		deleted += len(ids)
	}
	return deleted, nil
}
