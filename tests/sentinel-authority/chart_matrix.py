#!/usr/bin/env python3
"""Run the repository's chart install/test commands only on a guarded disposable cluster."""
import argparse
import json
import pathlib

import verify


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-charts')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--helm', default='helm')
    args = p.parse_args()
    c = verify.Cluster(args)
    try:
        c.get('namespace', c.ns, namespace=False)
    except RuntimeError as error:
        if 'NotFound' not in str(error): raise
    else:
        raise ValueError('refusing to reuse a chart-test namespace')
    c.apply({'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': c.ns, 'labels': {'verification.redis.opstreelabs.in/owner': verify.OWNER}}})
    installed = []
    common = ['--kubeconfig', str(args.kubeconfig), '--kube-context', f'kind-{args.cluster}']
    try:
        # Use the already installed local candidate, never install a second
        # cluster-scoped controller that could race it over these resources.
        verify.command([args.helm, 'test', 'redis-operator', '--namespace', 'redis-operator-system', *common])
        for chart in ('redis', 'redis-cluster', 'redis-replication', 'redis-sentinel'):
            verify.command([args.helm, 'install', chart, str(verify.ROOT / 'charts' / chart), '--namespace', c.ns, *common], timeout=120)
            installed.append(chart)
            verify.command([args.helm, 'test', chart, '--namespace', c.ns, *common], timeout=180)
            hooks = verify.command([args.helm, 'get', 'hooks', chart, '--namespace', c.ns, *common])
            print(json.dumps({'chart': chart, 'installed': True, 'helm_test_passed': True, 'hook_manifest_present': bool(hooks.strip()), 'runtime_failover_qualified': False}), flush=True)
    finally:
        for chart in reversed(installed):
            verify.command([args.helm, 'uninstall', chart, '--namespace', c.ns, '--wait', '--timeout', '120s', *common], timeout=150)
        c.require_owner()
        c.k('delete', 'namespace', c.ns, '--wait=true', '--timeout=120s', namespace=False, timeout=150)


if __name__ == '__main__':
    main()
