# redis-backup

Scheduled or one-shot backups of a `RedisCluster` managed by the OT-CONTAINER-KIT
redis-operator. Backups use `redis-cli --rdb` against each cluster leader and push the
resulting `dump.rdb` files to object storage (S3, Azure Blob, or GCS) via `restic`.

The chart wraps the existing `scripts/backup/backup.bash` flow — previously only
available as copy-paste YAML under `example/v1beta2/backup_restore/` — into a chart
with scoped RBAC and a CronJob path.

**Homepage:** <https://github.com/ot-container-kit/redis-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| iamabhishek-dubey |  |  |
| sandy724 |  |  |
| shubham-cmyk |  |  |

## Pre-Requisites

- Kubernetes 1.21+ (1.27+ recommended if using `cronJob.timeZone`)
- Helm 3.X
- Redis Operator installed, with a `RedisCluster` CR to back up
- Credentials for the object storage destination (S3 / Azure Blob / GCS)

## Source Code

* <https://github.com/ot-container-kit/redis-operator>

## Install

The chart is **opt-in**: default values render zero resources. Enable it, name the
target cluster, and point it at a Secret with the restic + destination credentials.

```shell
# Scheduled: nightly CronJob
helm install my-backup ot-helm/redis-backup \
  --namespace redis \
  --set enabled=true \
  --set clusterName=my-cluster \
  --set schedule="0 2 * * *" \
  --set existingSecret=my-backup-creds

# One-shot: single Job fires immediately
helm install my-backup ot-helm/redis-backup \
  --namespace redis \
  --set enabled=true \
  --set clusterName=my-cluster \
  --set existingSecret=my-backup-creds
```

If `schedule` is set the chart renders a `CronJob`; otherwise it renders a single
`Job`. The one-shot path is useful for ad-hoc backups and for migrating off the
example-YAML flow.

### Expected keys in the credentials Secret

`envFrom` is used to inject every key in the Secret as an env var on the backup
container, so the Secret must contain the keys that `scripts/backup/backup.bash`
reads:

| Key | Required | Notes |
| --- | --- | --- |
| `RESTIC_PASSWORD` | yes | Password for the restic repository |
| `RESTIC_CACHE_DIR` | no | Defaults to `/tmp/restic_cache` |
| `BACKUP_DESTINATION` | yes | One of `AWS_S3`, `AZURE_BLOB`, `GOOGLE_CLOUD` |
| `REDIS_PORT` | yes | e.g. `"6379"` |
| `REDIS_PASSWORD` | no | Empty for password-less clusters |
| `AWS_S3_BUCKET`, `AWS_DEFAULT_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | S3 only | |
| `AZURE_CONTAINER`, `AZURE_ACCOUNT_NAME`, `AZURE_ACCOUNT_KEY` | Azure only | |
| `GCP_BUCKET`, `GOOGLE_APPLICATION_CREDENTIALS` | GCS only | |

In production, create this Secret out of band and reference it via `existingSecret`.
`secret.create: true` is intended for quick-start / local testing only; it writes
credentials into `values.yaml`.

## Trigger an ad-hoc run from an existing CronJob

```shell
kubectl create job --from=cronjob/my-backup-redis-backup \
  manual-$(date +%s) -n redis
```

## Uninstall

```shell
helm uninstall my-backup --namespace redis
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| clusterName | string | `""` | Name of the RedisCluster CR to back up. Required when enabled=true. |
| clusterNamespace | string | `""` | Namespace of the RedisCluster. Empty = the chart's release namespace. |
| cronJob | object | `{"concurrencyPolicy":"Forbid","failedJobsHistoryLimit":1,"startingDeadlineSeconds":null,"successfulJobsHistoryLimit":3,"suspend":false,"timeZone":""}` | CronJob-only tuning. Ignored when `schedule` is empty. |
| cronJob.startingDeadlineSeconds | string | `nil` | startingDeadlineSeconds; null = unset |
| cronJob.timeZone | string | `""` | IANA time zone (Kubernetes 1.27+). Empty = cluster default. |
| enabled | bool | `false` | Master switch. When false the chart renders no resources, so `helm install`    with defaults is a safe no-op. |
| existingSecret | string | `""` | Reference an existing Secret with the backup env vars. Preferred for    production. Ignored when `secret.create: true`. |
| extraEnv | list | `[]` | Extra env to append after envFrom (useful for toggles that aren't secrets). |
| extraVolumeMounts | list | `[]` |  |
| extraVolumes | list | `[]` | Extra volumes mounted into the backup pod (e.g. a restic cache PVC). |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"quay.io/opstree/redis-operator-backup"` |  |
| image.tag | string | `"latest"` |  |
| imagePullSecrets | list | `[]` |  |
| job | object | `{"activeDeadlineSeconds":null,"backoffLimit":2,"restartPolicy":"OnFailure","ttlSecondsAfterFinished":86400}` | Job tuning. Applies to the one-shot Job and to each CronJob invocation. |
| job.activeDeadlineSeconds | string | `nil` | Null = unset |
| job.ttlSecondsAfterFinished | int | `86400` | Garbage-collect finished Jobs after this many seconds. Null = never. |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext.fsGroup | int | `1000` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.runAsUser | int | `1000` |  |
| rbac.create | bool | `true` | Create a scoped Role + RoleBinding granting `get` on redisclusters.    backup.bash looks up the cluster's leader count from the CR, so this is    the minimum needed. Disable if you've bound the SA to a Role yourself. |
| resources | object | `{}` |  |
| schedule | string | `""` | Cron schedule in standard cron syntax. Empty = render a one-shot Job    instead of a CronJob. |
| secret | object | `{"create":false,"name":"","stringData":{}}` | Secret providing backup env vars (restic + destination credentials). Mounted    with envFrom, so every key becomes an env var on the backup container.    Choose exactly one path: `secret.create: true` (chart creates it from    stringData) or `existingSecret: <name>` (bring your own). |
| secret.create | bool | `false` | Create a Secret from `stringData` below. Keep secrets out of values.yaml    in real deployments; prefer `existingSecret`. |
| secret.name | string | `""` | Override the generated Secret name. Default: <release>-redis-backup |
| secret.stringData | object | `{}` | Free-form env map written into the generated Secret. |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| securityContext.readOnlyRootFilesystem | bool | `false` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` | Override the generated ServiceAccount name. Default: <release>-redis-backup |
| tolerations | list | `[]` |  |
