#!/usr/bin/env python3
"""Exercise kubelet cancellation of an established, unresponsive readiness connection."""
import argparse
import json
import pathlib
import time

import verify


SERVER = r'''
import os, pathlib, socket, threading, time
counter = 0
def handle(conn):
    global counter
    with conn:
        payload = conn.recv(4096)
        counter += 1
        pathlib.Path('/control/accepted').write_text(str(counter))
        while not pathlib.Path('/control/reply').exists():
            time.sleep(0.05)
        if b'hello' in payload.lower():
            try:
                conn.sendall(b"-ERR unknown command 'hello'\r\n")
                conn.settimeout(2)
                if not conn.recv(4096):
                    return
            except OSError:
                return
        fields = ['ip', os.environ['OWN_IP'], 'port', '6379', 'flags', 'master', 'config-epoch', '1']
        response = '*' + str(len(fields)) + '\r\n'
        for field in fields:
            response += '$' + str(len(field)) + '\r\n' + field + '\r\n'
        try:
            conn.sendall(response.encode())
        except OSError:
            pass
server = socket.socket()
server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server.bind(('127.0.0.1', 26379))
server.listen()
while True:
    conn, _ = server.accept()
    threading.Thread(target=handle, args=(conn,), daemon=True).start()
'''


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig', type=pathlib.Path, required=True)
    p.add_argument('--cluster', required=True)
    p.add_argument('--namespace', default='authority-test-probes')
    p.add_argument('--kubectl', default='kubectl')
    p.add_argument('--hold-seconds', type=int, default=45)
    p.add_argument('--probe-image', default='quay.io/opstree/redis:v8.4.0')
    p.add_argument('--timeout-seconds', type=int, choices=(1, 2), default=2)
    args = p.parse_args()
    if not 20 <= args.hold_seconds <= 90:
        p.error('hold must cover repeated cancellations, between 20 and 90 seconds')
    c = verify.Cluster(args)
    c.require_owner()
    name = 'readiness-timeout-probe'
    try:
        c.get('pod', name)
    except RuntimeError as error:
        if 'NotFound' not in str(error):
            raise
    else:
        raise ValueError('refusing to reuse an existing probe')
    # This generated shell script contains no credential values. Never read the
    # adjacent redis.conf or a Sentinel runtime configuration for this test.
    def live_script():
        for pod in c.pods():
            if not any(status.get('state', {}).get('running') for status in pod['status'].get('containerStatuses', [])):
                continue
            script = c.k('exec', pod['metadata']['name'], '--', 'cat', '/etc/redis/sentinel-ready.sh')
            if args.probe_image != 'quay.io/opstree/redis:v8.4.0' and 'exec /etc/redis/sentinel-ready agent readiness' not in script:
                continue
            return script
        return False
    script = verify.wait(live_script, 'readiness script available from a running member')
    c.apply({'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': name, 'namespace': c.ns}, 'data': {'ready.sh': script}})
    own_ip = {'name': 'SENTINEL_MEMBER_IP', 'valueFrom': {'fieldRef': {'fieldPath': 'status.podIP'}}}
    c.apply({'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name, 'namespace': c.ns}, 'spec': {
        'automountServiceAccountToken': False,
        'securityContext': {'runAsUser': 1000, 'fsGroup': 1000},
        'volumes': [{'name': 'probe', 'configMap': {'name': name}}, {'name': 'control', 'emptyDir': {}}],
        'containers': [
            {'name': 'redis', 'image': args.probe_image, 'imagePullPolicy': 'IfNotPresent',
             'command': ['redis-server', '--protected-mode', 'no', '--save', '', '--appendonly', 'no', '--dir', '/tmp'],
             'volumeMounts': [{'name': 'probe', 'mountPath': '/probe', 'readOnly': True}],
             'env': [own_ip, {'name': 'SENTINEL_BOOTSTRAP_ADDRS', 'value': '127.0.0.1'}],
             'readinessProbe': {'exec': {'command': ['sh', '/probe/ready.sh']}, 'timeoutSeconds': args.timeout_seconds, 'periodSeconds': 2, 'failureThreshold': 1}},
            {'name': 'blackhole', 'image': 'python:3.13-alpine', 'command': ['python', '-u', '-c', SERVER],
             'env': [{'name': 'OWN_IP', 'valueFrom': {'fieldRef': {'fieldPath': 'status.podIP'}}}],
             'volumeMounts': [{'name': 'control', 'mountPath': '/control'}]},
        ]}})
    def started():
        pod = c.get('pod', name)
        return len(pod['status'].get('containerStatuses', [])) == 2 and all('running' in x['state'] for x in pod['status']['containerStatuses'])
    verify.wait(started, 'isolated probe running')
    verify.wait(lambda: int(c.k('exec', name, '-c', 'blackhole', '--', 'cat', '/control/accepted')) > 0, 'established connection accepted')
    time.sleep(args.hold_seconds)
    def processes():
        rows = c.k('exec', name, '-c', 'redis', '--', 'ps', '-o', 'stat,comm').splitlines()
        return {'zombies': sum(row.split()[0].startswith('Z') for row in rows if row.split()),
                'redis_cli': sum(row.split()[-1] == 'redis-cli' for row in rows if row.split())}
    blocked = processes()
    probe_uid = c.get('pod', name)['metadata']['uid']
    events = c.get('events')['items']
    cancellations = sum(item.get('count', 1) for item in events if item.get('involvedObject', {}).get('uid') == probe_uid and 'timed out' in item.get('message', '').lower())
    pod = c.get('pod', name)
    assert not next(x for x in pod['status']['containerStatuses'] if x['name'] == 'redis')['ready']
    c.k('exec', name, '-c', 'blackhole', '--', 'python', '-c', "import pathlib; pathlib.Path('/control/reply').touch()")
    verify.wait(lambda: next(x for x in c.get('pod', name)['status']['containerStatuses'] if x['name'] == 'redis')['ready'], 'readiness recovered')
    time.sleep(5)
    recovered = processes()
    restarts = sum(x['restartCount'] for x in c.get('pod', name)['status']['containerStatuses'])
    passed = blocked['zombies'] == 0 and blocked['redis_cli'] <= 1 and recovered['zombies'] == 0 and recovered['redis_cli'] <= 1 and restarts == 0
    print(json.dumps({'established_read_timeout': True, 'hold_seconds': args.hold_seconds, 'kubelet_timeout_seconds': args.timeout_seconds, 'observed_cancellations': cancellations, 'blocked': blocked, 'recovered': recovered, 'restarts': restarts, 'passed': passed}), flush=True)
    if not passed:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
