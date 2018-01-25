/*
Copyright 2017 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"os"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
)

const rootPath = "spark-app-dependencies"

type gcsUploader struct {
	client *storage.Client
	handle *storage.BucketHandle
	bucket string
	path string
}

func newGcsUploader(bucket string, path string, projectID string, ctx context.Context) (*gcsUploader, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	handle := client.Bucket(bucket)
	// Check if the bucket exists.
	if _, err := handle.Attrs(ctx); err != nil {
		return nil, err
	}
	uploader := &gcsUploader{
		client: client,
		handle: handle.UserProject(projectID),
		bucket: bucket,
		path: path}

	return uploader, nil
}

func (g *gcsUploader) upload(ctx context.Context, localFile string, public bool) (string, error) {
	fmt.Printf("Uploading local file: %s\n", localFile)
	file, err := os.Open(localFile)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %v", localFile, err)
	}

	object := g.handle.Object(filepath.Join(g.path, filepath.Base(localFile)))
	writer := object.NewWriter(ctx)
	if _, err = io.Copy(writer, file); err != nil {
		return "", fmt.Errorf("failed to copy file %s to GCS: %v", localFile, err)
	}
	if err = writer.Close(); err != nil {
		return "", err
	}

	objectAttrs, err := object.Attrs(ctx)
	if err != nil {
		return "", err
	}

	if public {
		if err = object.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
			return "", fmt.Errorf("failed to set ACL on GCS object %s: %v", objectAttrs.Name, err)
		}
		return fmt.Sprintf("https://storage.googleapis.com/%s/%s", objectAttrs.Bucket, objectAttrs.Name), nil
	}
	return fmt.Sprintf("gs://%s/%s", objectAttrs.Bucket, objectAttrs.Name), nil
}

func uploadToGCS(
	bucket string,
	appNamespace string,
	appName string,
	projectID string,
	files []string,
	public bool) ([]string, error) {
	ctx := context.Background()
	uploader, err := newGcsUploader(bucket, filepath.Join(rootPath, appNamespace, appName), projectID, ctx)
	if err != nil {
		return nil, err
	}
	defer uploader.client.Close()

	var uploadedFiles []string
	for _, file := range files {
		remoteFile, err := uploader.upload(ctx, file, public)
		if err != nil {
			return nil, err
		}
		uploadedFiles = append(uploadedFiles, remoteFile)
	}

	return uploadedFiles, nil
}
