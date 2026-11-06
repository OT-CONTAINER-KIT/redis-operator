#!/usr/bin/env python3
"""Verify a full Redis cold start refuses to invent a primary; leave the fixture fenced."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import time

import verify


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-cold-start')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--backup-dir', type=pathlib.Path, help='new approved local artifact directory for this synthetic one-key dataset only')
    p.add_argument('--expected-init-image')
    args = p.parse_args()
    c = verify.Cluster(args)
    c.require_owner()
    primary = c.healthy()
    c.marker(primary)
    sources = {}
    if args.backup_dir:
        if not args.backup_dir.resolve().is_relative_to('/var/tmp/daniel/tmp/opencode') or args.backup_dir.exists():
            raise ValueError('backup requires a new directory under the approved local artifact root')
        assert all(c.cli(name, 'DBSIZE') == '1' for name in c.ids()), 'export is restricted to the controlled one-key fixture'
        for pod in c.pods():
            main = pod['spec']['containers'][0]
            data_volume = next(m['name'] for m in main['volumeMounts'] if m['mountPath'] == '/data')
            claim = next(v['persistentVolumeClaim']['claimName'] for v in pod['spec']['volumes'] if v['name'] == data_volume)
            sources[pod['metadata']['name']] = {'claim': claim, 'node': pod['spec']['nodeName']}
    claims = {v['metadata']['name']: v['metadata']['uid'] for v in c.get('pvc')['items']}
    assert len(claims) == 6
    voters = {name: c.cli(name, 'SENTINEL', 'MYID', sentinel=True) for name in c.ids(True)}
    old_ids = c.ids()
    # This is deliberately cluster-wide and must be the last scenario in this
    # disposable cluster. No shared-cluster/context fallback is accepted.
    c.k('-n', 'redis-operator-system', 'scale', 'deployment', 'redis-operator', '--replicas=0', namespace=False)
    verify.wait(c.operator_stopped, 'operator actually terminated in its own namespace')
    c.k('scale', 'statefulset', 'redis-replication', '--replicas=0')
    verify.wait(lambda: not c.pods(), 'all Redis pods terminated before cold start')
    c.k('scale', 'statefulset', 'redis-replication', '--replicas=3')
    try:
        verify.wait(lambda: any(p['metadata']['uid'] not in old_ids.values() and p['status'].get('initContainerStatuses') for p in c.pods()), 'replacement initializer attempted startup')
        def authority_refused():
            for pod in c.pods():
                log = c.k('logs', pod['metadata']['name'], '-c', 'init-config')
                if any(message in log for message in ('waiting for Sentinel majority', 'waiting for the Sentinel primary to confirm its role', 'sentinel startup source is not a current RedisReplication member')):
                    return True
            return False
        verify.wait(authority_refused, 'initializer explicitly refused unavailable authority')
        samples = 0
        end = time.monotonic() + 45
        while time.monotonic() < end:
            pods = c.pods()
            assert pods and all(not p['status'].get('containerStatuses') or all(not x.get('started') and 'running' not in x.get('state', {}) for x in p['status']['containerStatuses']) for p in pods), 'a Redis main process started without authority'
            assert not c.endpoints(), 'cold Redis was advertised as writable'
            samples += 1
            time.sleep(3)
        after = {v['metadata']['name']: v['metadata']['uid'] for v in c.get('pvc')['items']}
        assert after == claims
        assert {name: c.cli(name, 'SENTINEL', 'MYID', sentinel=True) for name in c.ids(True)} == voters
        print(json.dumps({'cold_start_fail_closed': True, 'authority_guard_observed': True, 'observation_seconds': 45, 'samples': samples, 'state_claims_retained': True, 'voter_identities_retained': True, 'automatic_recovery': False, 'operator_left_stopped': True, 'passed': True}), flush=True)
    finally:
        # No automatic unfencing, role change, monitor reset, or data deletion.
        c.k('scale', 'statefulset', 'redis-replication', '--replicas=0')
        verify.wait(lambda: not c.pods(), 'cold-start fixture fenced again')

    if args.backup_dir:
        args.backup_dir.mkdir(mode=0o700)
        digests = {}
        for name, source in sources.items():
            helper = 'offline-copy-' + name
            c.apply({'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': helper, 'namespace': c.ns}, 'spec': {
                'automountServiceAccountToken': False, 'nodeName': source['node'],
                'securityContext': {'runAsUser': 1000, 'fsGroup': 1000},
                'containers': [{'name': 'copy', 'image': 'quay.io/opstree/redis:v8.4.0', 'command': ['sleep', '600'], 'volumeMounts': [{'name': 'source', 'mountPath': '/source', 'readOnly': True}]}],
                'volumes': [{'name': 'source', 'persistentVolumeClaim': {'claimName': source['claim'], 'readOnly': True}}]}})
            try:
                verify.wait(lambda: c.get('pod', helper)['status']['phase'] == 'Running', 'offline readonly copy mounted')
                archive = args.backup_dir / (name + '.tar')
                with archive.open('xb') as output:
                    result = subprocess.run([*c.base, '-n', c.ns, 'exec', helper, '--', 'tar', '-cf', '-', '-C', '/source', '.'], stdout=output, stderr=subprocess.PIPE, timeout=60)
                assert result.returncode == 0, 'offline dataset export failed'
                archive.chmod(0o600)
                with archive.open('rb') as data:
                    digests[archive.name] = hashlib.file_digest(data, 'sha256').hexdigest()
            finally:
                c.k('delete', 'pod', helper, '--wait=true', '--timeout=60s')
        receipt = {'synthetic_one_key_only': True, 'old_cluster': args.cluster, 'old_namespace': c.ns,
                   'primary_before_outage': primary, 'source_members': sources, 'old_voter_ids': sorted(voters.values()),
                   'old_writers_stopped': not c.pods(), 'operator_stopped': True, 'archive_sha256': digests, 'source_claim_uids': claims}
        (args.backup_dir / 'receipt.json').write_text(json.dumps(receipt, indent=2))
        print(json.dumps({'offline_dataset_archives': len(sources), 'source_mounts_read_only': True, 'dataset_writes_requested': False, 'sentinel_state_archived': False}), flush=True)


if __name__ == '__main__':
    main()
