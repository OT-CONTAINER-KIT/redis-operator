#!/usr/bin/env python3
"""Compare a source-matched native connection factory and reconnect control through election."""
import argparse
import json
import pathlib
import time

import verify


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-node-clients')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--client-image', required=True)
    p.add_argument('--expected-init-image')
    args = p.parse_args()
    c = verify.Cluster(args)
    c.require_owner()
    c.healthy()
    name = 'native-client-probe'
    try:
        c.get('pod', name)
    except RuntimeError as error:
        if 'NotFound' not in str(error): raise
    else:
        raise ValueError('refusing to reuse a completed client probe')
    c.apply({'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name, 'namespace': c.ns}, 'spec': {
        'automountServiceAccountToken': False, 'restartPolicy': 'Never',
        'containers': [{'name': 'probe', 'image': args.client_image, 'imagePullPolicy': 'IfNotPresent'}]}})
    verify.wait(lambda: '"native_clients_started":true' in c.k('logs', name), 'both native clients connected')
    time.sleep(10)
    elected = c.failover()
    verify.wait(lambda: c.get('pod', name)['status']['phase'] in ('Succeeded', 'Failed'), 'native client probe finished', 120)
    rows = [json.loads(line) for line in c.k('logs', name).splitlines() if line.startswith('{')]
    result = next(row for row in rows if 'native_client_results' in row)
    assert result['control_recovered'], 'positive reconnect control did not recover'
    print(json.dumps({'failover': elected, **result, 'operator_health_afterward': bool(c.healthy())}), flush=True)


if __name__ == '__main__':
    main()
