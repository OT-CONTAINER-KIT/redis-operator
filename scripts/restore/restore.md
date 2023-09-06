# Restore Redis from S3, Google Cloud Storage, or Azure Blob

Follow the steps below to restore a Redis backup from Amazon S3, Google Cloud Storage, or Azure Blob.

## Prerequisites

- Credentials and access to an S3 bucket, Google Cloud Storage or azure blob.

## Steps

### 1. Set Up the Restore Environment

- First, create a Docker image using the `Dockerfile.restore`. This image will encompass all necessary tools for the restoration process.
  
- Ensure that the `restore.bash` script is included within the `Dockerfile.restore` to be available in the container.

### 2. Manage Environment Variables through Kubernetes Secrets

- For a more secure approach, utilize Kubernetes secrets to manage and pass your environment variables.

- The template for the necessary environment variables can be found at `./restore/env_vars.env`.

> Note : You have to pass image in the init container to backup the redis data. Since dump.rbd file should be loaded before the redis server starts.
