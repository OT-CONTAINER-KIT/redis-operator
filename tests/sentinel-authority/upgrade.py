#!/usr/bin/env python3
"""Disposable-only legacy storage adoption and persistent-protocol rollback."""
import argparse
import json
import pathlib

import verify


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-upgrade')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--helm', default='helm')
    p.add_argument('--candidate-tag', required=True)
    p.add_argument('--compatible-tag', help='previous locally loaded persistence-aware image, never stock v0.23')
    p.add_argument('mode', choices=['upgrade', 'rollback', 'partial-rollback'])
    args = p.parse_args()
    if args.mode != 'upgrade' and not args.compatible_tag:
        p.error('rollback requires a persistence-aware compatible tag; stock v0.23 is not a supported image-only rollback')
    c = verify.Cluster(args)
    c.require_owner()
    for tag in dict.fromkeys(tag for tag in (args.candidate_tag, args.compatible_tag) if tag):
        image = f'redis-operator:{tag}'
        # Inspect never pulls. Help runs with no network or writable root and
        # does not initialize state; reject a stock/old agent before staging it.
        verify.command(['docker', 'image', 'inspect', '--format', '{{.Id}}', image])
        help_text = verify.command(['docker', 'run', '--rm', '--read-only', '--network=none', '--cap-drop=ALL', '--entrypoint', '/operator', image, 'agent', 'bootstrap', '--help'])
        if not all(flag in help_text for flag in ('--standalone-authority', '--persistent-sentinel-state')):
            raise ValueError('image does not implement the persistent standalone authority protocol')
    primary = c.healthy()
    c.marker(primary)
    original = c.ids()

    def election_state():
        claims = {p['metadata']['name']: p['metadata']['uid'] for p in c.get('pvc', selector='app=redis-sentinel-sentinel')['items']}
        voters = {f'redis-sentinel-sentinel-{i}': c.cli(f'redis-sentinel-sentinel-{i}', 'SENTINEL', 'MYID', sentinel=True) for i in range(3)}
        assert len(claims) == 3 and len(set(voters.values())) == 3
        return claims, voters, min(int(v['config-epoch']) for v in c.votes())

    retained = None

    def assert_retained():
        if retained is not None:
            claims, voters, epoch = election_state()
            assert (claims, voters) == retained[:2], 'Sentinel state claim or voter identity changed during image transition'
            assert epoch >= retained[2], 'Sentinel configuration epoch regressed'

    def partition(kind, name, count):
        c.k('patch', kind, name, '--type=merge', '-p', json.dumps({'spec': {'kubernetesConfig': {'updateStrategy': {'type': 'RollingUpdate', 'rollingUpdate': {'partition': count}}}}}))
        name = 'redis-replication' if kind == 'redisreplication' else 'redis-sentinel-sentinel'
        verify.wait(lambda: c.get('statefulset', name)['spec']['updateStrategy'].get('rollingUpdate', {}).get('partition') == count, f'{name} gate {count}')

    def helm(tag):
        verify.command([args.helm, 'upgrade', 'redis-operator', str(verify.ROOT / 'charts/redis-operator'), '--namespace', 'redis-operator-system', '--kubeconfig', str(args.kubeconfig), '--kube-context', f'kind-{args.cluster}', '--reset-values', '--set', 'redisOperator.imageName=redis-operator', '--set', f'redisOperator.imageTag={tag}', '--set', 'redisOperator.imagePullPolicy=IfNotPresent', '--wait', '--timeout', '120s'], timeout=180)
        def staged():
            for name in ('redis-sentinel-sentinel', 'redis-replication'):
                sts = c.get('statefulset', name)
                if sts.get('status', {}).get('observedGeneration', 0) < sts['metadata'].get('generation', 0):
                    return False
                if not any(init['name'] == 'init-config' and init['image'] == f'redis-operator:{tag}' for init in sts['spec']['template']['spec'].get('initContainers', [])):
                    return False
            return True
        verify.wait(staged, 'both desired templates observed at the requested init tag')

    def rollout(kind, resource, prefix, tag_label):
        for index in (2, 1, 0):
            sentinel = kind == 'redissentinel'
            before = c.ids(sentinel)
            name = f'{prefix}-{index}'
            revision = c.get('statefulset', prefix)['status']['updateRevision']
            already = c.get('pod', name)['metadata']['labels'].get('controller-revision-hash') == revision
            partition(kind, resource, index)
            if not already:
                verify.wait(lambda: c.ids(sentinel).get(name) not in (None, before[name]), 'member replaced')
            selected = c.healthy()
            c.assert_marker(selected)
            assert_retained()
            if sentinel:
                assert c.ids() == original, 'Redis rolled while its partition gate was held'
            print(json.dumps({'stage': tag_label, 'ordinal': index, 'already_at_revision': already, 'primary': selected, 'marker_retained': True}), flush=True)

    partition('redisreplication', 'redis-replication', 3)
    partition('redissentinel', 'redis-sentinel', 3)
    stored = c.get('statefulset', 'redis-sentinel-sentinel')
    persistent = any(claim['metadata']['name'] == 'sentinel-state' for claim in stored['spec'].get('volumeClaimTemplates', []))
    if persistent:
        retained = election_state()
    if not persistent:
        if args.mode == 'rollback':
            raise ValueError('a persistent-protocol rollback requires existing durable Sentinel claims')
        old_ids = {c.cli(f'redis-sentinel-sentinel-{i}', 'SENTINEL', 'MYID', sentinel=True) for i in range(3)}
        # Freeze the legacy writer before adding fields its old Go type cannot
        # preserve. These effects are permitted only after Cluster's node guard.
        c.k('-n', 'redis-operator-system', 'scale', 'deployment', 'redis-operator', '--replicas=0', namespace=False)
        verify.wait(lambda: not json.loads(c.k('-n', 'redis-operator-system', 'get', 'pods', '-o', 'json', namespace=False))['items'], 'old operator stopped')
        c.k('apply', '--server-side', '--force-conflicts', '--field-manager=authority-test', '-f', str(verify.ROOT / 'config/crd/bases'), namespace=False)
        c.k('patch', 'redissentinel', 'redis-sentinel', '--type=merge', '-p', json.dumps({'spec': {'storage': {'volumeClaimTemplate': {'spec': {'storageClassName': 'standard', 'accessModes': ['ReadWriteOnce'], 'resources': {'requests': {'storage': '1Gi'}}}}}}}))
        c.k('delete', 'statefulset', 'redis-sentinel-sentinel', '--cascade=foreground', '--wait=true', '--timeout=120s', timeout=150)
        verify.wait(lambda: not c.pods(True), 'all legacy Sentinels terminated')
        assert c.ids() == original
        helm(args.candidate_tag)
        selected = c.healthy()
        assert c.ids() == original
        c.assert_marker(selected)
        new_ids = {c.cli(f'redis-sentinel-sentinel-{i}', 'SENTINEL', 'MYID', sentinel=True) for i in range(3)}
        assert old_ids.isdisjoint(new_ids), 'fresh state must not reuse a legacy voter identity'
        retained = election_state()
        print(json.dumps({'stage': 'legacy-state-adoption', 'old_voters_stopped': True, 'new_voter_set': True, 'redis_uids_preserved': True, 'marker_retained': True}), flush=True)
    else:
        helm(args.compatible_tag if args.mode == 'rollback' else args.candidate_tag)

    if args.mode == 'partial-rollback':
        # Actually replace one Sentinel before reverting; Redis remains gated.
        before = c.ids(True)
        revision = c.get('statefulset', 'redis-sentinel-sentinel')['status']['updateRevision']
        already = c.get('pod', 'redis-sentinel-sentinel-2')['metadata']['labels'].get('controller-revision-hash') == revision
        partition('redissentinel', 'redis-sentinel', 2)
        if not already:
            verify.wait(lambda: c.ids(True).get('redis-sentinel-sentinel-2') not in (None, before['redis-sentinel-sentinel-2']), 'one candidate Sentinel replaced')
        c.assert_marker(c.healthy())
        assert_retained()
        assert c.ids() == original
        print(json.dumps({'stage': 'partial-candidate-sentinel', 'ordinal': 2, 'already_at_revision': already, 'redis_uids_preserved': True}), flush=True)
        partition('redissentinel', 'redis-sentinel', 3)
        helm(args.compatible_tag)
    rollout('redissentinel', 'redis-sentinel', 'redis-sentinel-sentinel', 'sentinel-member')
    claims = {p['metadata']['name']: p['metadata']['uid'] for p in c.get('pvc', selector='app=redis-sentinel-sentinel')['items']}
    assert len(claims) == 3
    rollout('redisreplication', 'redis-replication', 'redis-replication', 'redis-member')
    after = {p['metadata']['name']: p['metadata']['uid'] for p in c.get('pvc', selector='app=redis-sentinel-sentinel')['items']}
    assert after == claims
    assert_retained()
    print(json.dumps({'completed': args.mode, 'passed': True, 'sentinel_claims_retained': True, 'voter_identities_retained': True, 'configuration_epoch_retained': True, 'crds_downgraded': False, 'legacy_image_rollback_supported': False}), flush=True)


if __name__ == '__main__':
    main()
