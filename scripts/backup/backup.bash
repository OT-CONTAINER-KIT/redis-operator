#!/bin/bash

set -e  # Exit on error

case "$BACKUP_DESTINATION" in
    "aws_s3"|"AWS_S3")
        RESTIC_REPOSITORY="s3:s3.${AWS_DEFAULT_REGION}.amazonaws.com/${AWS_S3_BUCKET}/${CLUSTER_NAME}-${CLUSTER_NAMESPACE}"
        ;;
    "azure_blob"|"AZURE_BLOB")
        RESTIC_REPOSITORY="azure:${AZURE_CONTAINER}:${CLUSTER_NAME}-${CLUSTER_NAMESPACE}"
        ;;
    "google_cloud"|"GOOGLE_CLOUD")
        RESTIC_REPOSITORY="gs:${GCP_BUCKET}/${CLUSTER_NAME}-${CLUSTER_NAMESPACE}"
        ;;
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
esac


REDIS_HOST="${CLUSTER_NAME}-leader-0.${CLUSTER_NAME}-leader-headless.${CLUSTER_NAMESPACE}.svc"

# Check the Total Leader Present in Redis Cluster using cr and redis-cli
TOTAL_LEADERS=$(kubectl get redisclusters.redis.redis.opstreelabs.in "${CLUSTER_NAME}" -n "${CLUSTER_NAMESPACE}" -o jsonpath='{.spec.redisLeader.replicas}')
MASTERS_IP=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" cluster nodes --no-auth-warning | grep "master" | awk '{print $2}' | cut -d "@" -f1)

check_total_leaders_from_cr() {
  # Check if TOTAL_LEADERS is 0 or nil
  if [[ -z "$TOTAL_LEADERS" || "$TOTAL_LEADERS" == 0 ]]; then
    echo "Error: Total number of leader pods is 0"
    exit 1
  fi
}

check_total_masters_from_redis() {
  # IF the total master by the redis custom-resource and the redis-cli doesn't match throw an error
  if [[ "${TOTAL_LEADERS}" -ne "${#MASTERS_IP[@]}" ]]; then
    echo "Error: Total number of leaders (${TOTAL_LEADERS}) is not equal to total number of masters (${#MASTERS_IP[@]})!"
    exit 1
  fi
}


initialize_repository() {
    # To set the password of the repo you must pass it the env Variable  RESTIC_PASSWORD
    if ! restic -r "$RESTIC_REPOSITORY" snapshots &>/dev/null ; then
        echo "Initializing restic repository..."
        restic init --repo "$RESTIC_REPOSITORY"
    else
        echo "Restic repository already initialized."
    fi
}


perform_redis_backup(){
    # Start performing backup
    for ((i = 0; i < TOTAL_LEADERS; i++))
    do  
        # Get the name of the Redis pod
        POD="${CLUSTER_NAME}-leader-${i}"

        # Get the IP address and port of the Redis instance
        IP_PORT="${MASTERS_IP[${i}]}"

        # Extract the IP address and port from the IP_PORT variable
        IP=$(echo "$IP_PORT" | cut -d ':' -f 1)
        PORT=$(echo "$IP_PORT" | cut -d ':' -f 2)

        echo "Performing backup on Redis instance at $IP:$PORT"

        # Copy the file from the Redis instance to a local file
        redis-cli -h "$IP" -p "$PORT" -a "$REDIS_PASSWORD" --rdb "/tmp/${POD}.rdb"

        # Upload the file to the selected backup destination using restic
        restic -r "$RESTIC_REPOSITORY" backup "/tmp/${POD}.rdb" --host"${CLUSTER_NAME}_${CLUSTER_NAMESPACE}" --tag "${POD}" --tag "redis"

        # Clean up the local file
        rm "/tmp/${POD}.rdb"
    done
}


check_total_leaders_from_cr
check_total_masters_from_redis
initialize_repository
perform_redis_backup
echo "Backup completed successfully."
