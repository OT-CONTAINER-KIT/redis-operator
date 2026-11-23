#!/usr/bin/env python3
"""Restore only cold_start.py's controlled one-key fixture into a fresh namespace/generation."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import tarfile

import verify


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-restored')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--backup-dir', type=pathlib.Path, required=True)
    p.add_argument('--source-kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--expected-init-image')
    args = p.parse_args()
    if not args.backup_dir.resolve().is_relative_to('/var/tmp/daniel/tmp/opencode'):
        p.error('requires an approved local synthetic backup directory')
    receipt = json.loads((args.backup_dir / 'receipt.json').read_text())
    if receipt.get('synthetic_one_key_only') is not True or not receipt.get('old_writers_stopped') or not receipt.get('operator_stopped'):
        raise ValueError('not an eligible synthetic fenced-source receipt')
    old_args = argparse.Namespace(**vars(args))
    old_args.cluster = receipt['old_cluster']
    old_args.namespace = receipt['old_namespace']
    old_args.kubeconfig = args.source_kubeconfig
    old = verify.Cluster(old_args)
    old.require_owner()
    assert not old.pods(), 'original Redis writers must remain fenced'
    assert old.operator_stopped(), 'original controller must actually be terminated'
    c = verify.Cluster(args)
    if args.cluster == old_args.cluster:
        raise ValueError('restore requires a separate disposable cluster while the old operator remains stopped')
    try:
        c.get('namespace', c.ns, namespace=False)
    except RuntimeError as error:
        if 'NotFound' not in str(error): raise
    else:
        raise ValueError('refusing to restore over an existing namespace')
    c.apply({'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': c.ns, 'labels': {'verification.redis.opstreelabs.in/owner': verify.OWNER}}})
    # Every pre-outage member was verified to contain precisely the same one
    # known key and was synchronously saved. This is not a source-selection rule
    # for arbitrary, divergent or partly replicated production datasets.
    if receipt['primary_before_outage'] not in ('redis-replication-0', 'redis-replication-1', 'redis-replication-2'):
        raise ValueError('invalid synthetic source member')
    archive = args.backup_dir / (receipt['primary_before_outage'] + '.tar')
    if archive.is_symlink() or archive.resolve().parent != args.backup_dir.resolve():
        raise ValueError('archive must remain inside its synthetic backup directory')
    with archive.open('rb') as source:
        assert hashlib.file_digest(source, 'sha256').hexdigest() == receipt['archive_sha256'][archive.name], 'synthetic archive changed after export'
    with tarfile.open(archive) as data:
        for member in data.getmembers():
            path = pathlib.PurePosixPath(member.name)
            if path.is_absolute() or '..' in path.parts or not (member.isdir() or member.isfile()):
                raise ValueError('unsafe synthetic dataset archive member')
    workers = sorted(node['metadata']['name'] for node in c.get('nodes', namespace=False)['items'] if 'node-role.kubernetes.io/control-plane' not in node['metadata'].get('labels', {}))
    if len(workers) < 3:
        raise ValueError('restore rehearsal requires three disposable worker nodes')
    for ordinal in range(3):
        claim = f'redis-replication-redis-replication-{ordinal}'
        helper = f'restore-data-{ordinal}'
        c.apply({'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim', 'metadata': {'name': claim, 'namespace': c.ns}, 'spec': {'storageClassName': 'standard', 'accessModes': ['ReadWriteOnce'], 'resources': {'requests': {'storage': '1Gi'}}}})
        c.apply({'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': helper, 'namespace': c.ns}, 'spec': {
            'automountServiceAccountToken': False, 'securityContext': {'runAsUser': 1000, 'fsGroup': 1000},
            'nodeSelector': {'kubernetes.io/hostname': workers[ordinal]},
            'containers': [{'name': 'copy', 'image': 'quay.io/opstree/redis:v8.4.0', 'command': ['sleep', '600'], 'volumeMounts': [{'name': 'target', 'mountPath': '/target'}]}],
            'volumes': [{'name': 'target', 'persistentVolumeClaim': {'claimName': claim}}]}})
        try:
            verify.wait(lambda: c.get('pod', helper)['status']['phase'] == 'Running', 'new empty restore claim mounted')
            assert c.k('exec', helper, '--', 'sh', '-c', 'test -z "$(ls -A /target)"') == ''
            with archive.open('rb') as source:
                result = subprocess.run([*c.base, '-n', c.ns, 'exec', '-i', helper, '--', 'tar', '-xf', '-', '-C', '/target'], stdin=source, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=60)
            assert result.returncode == 0, 'synthetic restore failed'
        finally:
            c.k('delete', 'pod', helper, '--wait=true', '--timeout=60s')
    for kind, name, image in [('RedisReplication', 'redis-replication', 'redis'), ('RedisSentinel', 'redis-sentinel', 'redis-sentinel')]:
        spec = {'clusterSize': 3, 'podSecurityContext': {'runAsUser': 1000, 'fsGroup': 1000}, 'kubernetesConfig': {'image': f'quay.io/opstree/{image}:v8.4.0', 'imagePullPolicy': 'IfNotPresent', 'resources': {'requests': {'cpu': '50m', 'memory': '64Mi'}, 'limits': {'cpu': '500m', 'memory': '256Mi'}}}, 'storage': {'volumeClaimTemplate': {'spec': {'storageClassName': 'standard', 'accessModes': ['ReadWriteOnce'], 'resources': {'requests': {'storage': '1Gi'}}}}}}
        if kind == 'RedisSentinel':
            spec['redisSentinelConfig'] = {'redisReplicationName': 'redis-replication', 'masterGroupName': verify.GROUP, 'quorum': '2', 'downAfterMilliseconds': '10000', 'failoverTimeout': '60000', 'parallelSyncs': '1'}
        c.apply({'apiVersion': 'redis.redis.opstreelabs.in/v1beta2', 'kind': kind, 'metadata': {'name': name, 'namespace': c.ns}, 'spec': spec})
    primary = c.healthy()
    c.assert_marker(primary)
    assert all(c.cli(name, 'DBSIZE') == '1' for name in c.ids()), 'restored dataset differs from the one-key fixture'
    new_ids = {c.cli(name, 'SENTINEL', 'MYID', sentinel=True) for name in c.ids(True)}
    assert len(new_ids) == 3 and new_ids.isdisjoint(receipt['old_voter_ids'])
    assert not old.pods(), 'original writers must remain fenced after recovery'
    assert old.operator_stopped(), 'original controller must remain fenced after recovery'
    assert {claim['metadata']['name']: claim['metadata']['uid'] for claim in old.get('pvc')['items']} == receipt['source_claim_uids'], 'original claims changed during recovery'
    print(json.dumps({'cold_restore': True, 'originals_fenced': True, 'new_data_claims': 3, 'new_voter_generation': True, 'sentinel_state_restored': False, 'known_one_key_dataset_retained': True, 'arbitrary_dataset_selection_qualified': False, 'passed': True}), flush=True)


if __name__ == '__main__':
    main()
