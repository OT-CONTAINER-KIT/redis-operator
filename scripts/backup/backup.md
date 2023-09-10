# Backup Redis to S3/ Google Cloud/ AZURE BLOB

This guide will walk you through the process of backing up Redis to S3, Google Cloud or azure blob using Docker and Kubernetes tools.

## Prerequisites

- Credentials and access to an S3 bucket, Google Cloud Storage or azure blob.

## Steps

### 1. Select Your Backup Method

* For **Manual Backups**: Copy the backup-user.bash script in `Dockerfile.kubectl`
* For **Automated Backups** (using cronjobs/jobs): Use the backup.bash script in `Dockerfile.kubectl`

> 🚨 Important: If you're utilizing the backup.bash(Automated Backup) script, environment variables must be provided.

### 2. Set Up the Backup Environment

* Run the Dockerfile.kubectl image to create a pod with kubectl and other tools installed.
  
> The related manifest can be found at `./scripts/backup/manifest`

### 3. Configure the Environment Variables

For the job/cron you need to configure env You can achieve it as explained below:

* Create a file named `env_vars.env` in your current directory.
* Populate the file with necessary environment variables.
* Source the environment file to load the variables using below command.

```bash
source ./env_vars.env
```

> For a more secure approach, utilize Kubernetes secrets to manage and pass your environment variables.

You can refer to the example `env_vars.env` file located at `./scripts/backup/env_vars.env`.
