#!/bin/bash

set -e  # Exit on error

# Get the pod name and namespace
POD=$(hostname)
NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
DATA_DIR=${DATA_DIR:-"/data"}

# Check if the pod name ends with 'follower-[index]'
if [[ $POD =~ -follower-[0-9]+$ ]]; then
    echo "Pod name ends with 'follower-[index]', exiting with status code 0."
    exit 0
fi

# Set the RESTIC_REPOSITORY based on the BACKUP_DESTINATION
case "$BACKUP_DESTINATION" in
    "aws_s3"|"AWS_S3")
        RESTIC_REPOSITORY="s3:s3.${AWS_DEFAULT_REGION}.amazonaws.com/${AWS_S3_BUCKET}/${CLUSTER_NAME}-${NAMESPACE}"
        ;;
    "azure_blob"|"AZURE_BLOB")
        RESTIC_REPOSITORY="azure:${AZURE_CONTAINER}:${CLUSTER_NAME}-${NAMESPACE}"
        ;;
    "google_cloud"|"GOOGLE_CLOUD")
        RESTIC_REPOSITORY="gs:${GCP_BUCKET}/${CLUSTER_NAME}-${NAMESPACE}"
        ;;
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
esac


# Find the latest snapshot for the current pod
SNAPSHOT_ID=$(restic -r "$RESTIC_REPOSITORY" snapshots --json --tag "${POD}"  | jq -r 'max_by(.time) | .id')

if [ -z "$SNAPSHOT_ID" ]; then
  echo "Error: No snapshot found for the pod ${POD}"
  exit 1
fi
 

# Perform the restore
echo "Restoring Redis backup for pod ${POD} from snapshot ID ${SNAPSHOT_ID}"
restic -r "$RESTIC_REPOSITORY" restore "${SNAPSHOT_ID}" --target "/"

# Move the restored file to the correct location
mv "/tmp/${POD}.rdb" "${DATA_DIR}/dump.rdb"

# Change the ownership of the restored file
# chown redis:redis "${DATA_DIR}/dump.rdb"

echo "Restore completed successfully for pod ${POD}"
