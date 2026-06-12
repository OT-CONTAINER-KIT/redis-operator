# Migration Guide: v0.23.0 to v0.24.0

This guide covers breaking changes and required actions when upgrading from v0.23.0 to v0.24.0. Review each section and apply the steps relevant to your setup before upgrading.

---

## 1. Breaking Change: TLS CA Certificate Configuration

**Affected users:** Anyone with TLS enabled on Redis instances (standalone, cluster, replication, or sentinel) who relied on the default `TLS.ca` value.

**PR:** [#1644](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/1644)

### What Changed

The TLS CA file reference has been corrected from `ca.key` to `ca.crt` to properly point to the CA **certificate** instead of the CA **key**.

| Component | v0.23.0 (old) | v0.24.0 (new) |
|-----------|---------------|---------------|
| Helm chart `TLS.ca` default value | `ca.key` | `ca.crt` |
| Environment variable name | `REDIS_TLS_CA_KEY` | `REDIS_TLS_CA_CERT` |
| CRD struct field (Go API) | `CaKeyFile` | `CaCertFile` |

The JSON tag for the CRD field remains `ca`, so existing CR manifests using `ca:` in their YAML are unaffected at the API level.

### Who Is Affected

You are affected if **all** of the following are true:

- You have TLS enabled on your Redis instances.
- Your TLS secret stores the CA certificate under a key named `ca.key` (matching the old default).
- You did not explicitly set the `TLS.ca` field in your Helm values or CR spec.

If you use **cert-manager**, your secret likely already stores the CA certificate under `ca.crt`, meaning this upgrade will actually **fix** a previously broken configuration for you.

### Symptoms

After upgrading, Redis pods crash with:

```
*** FATAL CONFIG FILE ERROR (Redis x.x.x) ***
Reading the configuration file, at line XX
>>> 'tls-ca-cert-file' wrong number of arguments
```

### Migration Steps

**Option A: Update your TLS secret (recommended)**

Rename the CA certificate key in your Kubernetes secret from `ca.key` to `ca.crt`:

```shell
# Export the existing CA cert data
kubectl get secret <your-tls-secret> -n <namespace> -o jsonpath='{.data.ca\.key}' | base64 -d > /tmp/ca.crt

# Patch the secret to add ca.crt key
kubectl patch secret <your-tls-secret> -n <namespace> \
  --type='json' \
  -p='[{"op": "add", "path": "/data/ca.crt", "value": "'$(base64 -w0 /tmp/ca.crt)'"}]'

# Clean up
rm /tmp/ca.crt
```

Then upgrade the operator. Once confirmed working, you can optionally remove the old `ca.key` entry from the secret.

**Option B: Explicitly set the old value in your configuration**

If you cannot rename the secret key, explicitly set `TLS.ca` to `ca.key` in your Helm values or CR spec to preserve the old behavior:

For Helm values:

```yaml
TLS:
  ca: ca.key
  cert: tls.crt
  key: tls.key
```

For CR spec (applies to Redis, RedisCluster, RedisReplication, RedisSentinel):

```yaml
spec:
  TLS:
    ca: ca.key
    cert: tls.crt
    key: tls.key
    secret:
      secretName: <your-tls-secret>
```

---

## 2. Breaking Change: Webhook Configuration Name Includes Release Name Prefix

**Affected users:** Anyone with webhooks enabled (`redisOperator.webhook=true`).

**PR:** [#1651](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/1651)

### What Changed

The `MutatingWebhookConfiguration` resource name changed from the static name `mutating-webhook-configuration` to `<release-name>-mutating-webhook-configuration`. This prevents naming conflicts when multiple operator instances are installed in the same cluster.

### Why Action Is Required

Helm tracks resources by name. Since the name changed, `helm upgrade` will create the new webhook configuration but **will not delete the old one**. The stale old webhook will remain active, potentially intercepting requests with an outdated CA bundle or endpoint, causing admission failures.

### Migration Steps

Before or immediately after upgrading, delete the old webhook configuration:

```bash
# 1. Delete old webhook configuration
kubectl delete mutatingwebhookconfiguration mutating-webhook-configuration

# 2. Upgrade the operator
helm upgrade redis-operator ot-helm/redis-operator \
  --namespace <namespace> \
  --set redisOperator.webhook=true

# 3. Verify the new webhook exists
kubectl get mutatingwebhookconfiguration | grep redis-operator
```

---

## 3. Breaking Change: ACL PVC Mount Path Changed

**Affected users:** Anyone using ACL configuration from a PersistentVolumeClaim (`spec.acl.persistentVolumeClaim`).

**PR:** [#1645](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/1645)

### What Changed

Previously, the ACL PVC was mounted as a single file via `subPath` at `/etc/redis/user.acl`. This caused `ACL SAVE` to fail with `Resource busy` because Linux forbids `rename(2)` on bind-mounted files (which is how Redis persists ACL changes).

In v0.24.0, the PVC is now mounted as a **directory** at `/data/redis`, and Redis reads/writes ACLs at `/data/redis/user.acl`.

| Component | v0.23.0 (old) | v0.24.0 (new) |
|-----------|---------------|---------------|
| ACL PVC mount path | `/etc/redis/user.acl` (subPath) | `/data/redis/` (directory) |
| ACL file location | `/etc/redis/user.acl` | `/data/redis/user.acl` |
| New env var `ACL_FILE_PATH` | not set | `/data/redis/user.acl` or `/etc/redis/user.acl` |
| Feature gate required | No | `GenerateConfigInInitContainer` must be enabled |

> **Note:** ACL from Secret (`spec.acl.secret`) is **not affected** — it still mounts at `/etc/redis/user.acl` via subPath.

### Why Action Is Required

- If your PVC has the `user.acl` file at the root, it will be picked up automatically from the new path.
- The `GenerateConfigInInitContainer` feature gate must be enabled for this feature to work correctly.
- Any custom scripts or tooling referencing the old path `/etc/redis/user.acl` for PVC-based ACLs must be updated.

### Migration Steps

1. Ensure the `GenerateConfigInInitContainer` feature gate is enabled on the operator.

2. Verify your ACL PVC has `user.acl` at its root directory (it will be mounted at `/data/redis/user.acl`).

3. Update any external tooling or scripts that reference the old path:
   ```
   Old: /etc/redis/user.acl
   New: /data/redis/user.acl
   ```

4. After upgrading, verify ACL SAVE works:
   ```bash
   kubectl exec -it <redis-pod> -n <namespace> -- redis-cli ACL SAVE
   ```

---

## Rollback

If you encounter issues after upgrading, downgrade back to v0.23.0:

```shell
helm upgrade redis-operator ot-helm/redis-operator \
  --namespace ot-operators --version 0.23.0
```

If you had deleted the old webhook configuration (section 2), it will be recreated automatically by Helm during the rollback since v0.23.0 uses the static name.
