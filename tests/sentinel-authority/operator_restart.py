#!/usr/bin/env python3
"""Check an authenticated fixture keeps its primary, voters and data across operator restart."""
import argparse
import json
import pathlib
import time

import verify


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-auth-latest')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--expected-init-image', required=True)
    args = p.parse_args()
    c = verify.Cluster(args)
    c.require_owner()
    assert c.get('redisreplication', 'redis-replication')['spec']['kubernetesConfig'].get('redisSecret'), 'requires the authenticated fixture'
    primary = c.healthy()
    c.marker(primary)
    redis_ids, sentinel_ids = c.ids(), c.ids(True)
    epoch = [vote['config-epoch'] for vote in c.votes()]
    voters = {name: c.cli(name, 'SENTINEL', 'MYID', sentinel=True) for name in sentinel_ids}
    c.k('-n', 'redis-operator-system', 'rollout', 'restart', 'deployment', 'redis-operator', namespace=False)
    c.k('-n', 'redis-operator-system', 'rollout', 'status', 'deployment/redis-operator', '--timeout=120s', namespace=False, timeout=150)
    end = time.monotonic() + 60
    samples = 0
    while time.monotonic() < end:
        assert c.healthy() == primary, 'operator restart caused a primary change'
        assert c.ids() == redis_ids and c.ids(True) == sentinel_ids
        assert [vote['config-epoch'] for vote in c.votes()] == epoch
        c.assert_marker(primary)
        samples += 1
        time.sleep(5)
    assert {name: c.cli(name, 'SENTINEL', 'MYID', sentinel=True) for name in sentinel_ids} == voters
    print(json.dumps({'operator_restart': True, 'authenticated': True, 'primary_unchanged': True, 'pod_uids_unchanged': True, 'voter_ids_unchanged': True, 'configuration_epoch_unchanged': True, 'marker_retained': True, 'samples': samples, 'passed': True}), flush=True)


if __name__ == '__main__':
    main()
