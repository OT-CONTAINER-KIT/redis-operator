#!/usr/bin/env python3
"""Opt-in destructive tests, restricted to an explicitly named disposable kind cluster."""
import argparse
import json
import pathlib
import secrets
import subprocess
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]
GROUP = "myMaster"
OWNER = "sentinel-authority-verification"


def command(args, payload=None, timeout=60):
    result = subprocess.run(args, input=payload, text=True, capture_output=True, timeout=timeout)
    if result.returncode:
        # Do not echo command input, manifests, or credential-bearing resources.
        raise RuntimeError(f"{args[0]} failed ({result.returncode}): {result.stderr[-1600:]}")
    return result.stdout


def wait(check, label, seconds=240):
    deadline = time.monotonic() + seconds
    last = None
    while time.monotonic() < deadline:
        try:
            result = check()
            if result:
                return result
        except (RuntimeError, subprocess.TimeoutExpired, KeyError, ValueError) as error:
            last = str(error)
        time.sleep(2)
    raise RuntimeError(f"timed out: {label}; last error: {last}")


class Cluster:
    def __init__(self, args):
        self.args = args
        self.ns = args.namespace
        self.base = [args.kubectl, "--kubeconfig", str(args.kubeconfig), "--context", f"kind-{args.cluster}"]
        if not args.cluster.startswith("redis-authority-verify-") or not self.ns.startswith("authority-test-"):
            raise ValueError("requires disposable redis-authority-verify-* cluster and authority-test-* namespace")
        nodes = self.get("nodes", namespace=False)["items"]
        if not nodes or not all(n["metadata"]["name"].startswith(args.cluster + "-") for n in nodes):
            raise ValueError("node identities do not match the disposable cluster")

    def k(self, *args, namespace=True, payload=None, timeout=60):
        return command([*self.base, *(["-n", self.ns] if namespace else []), *args], payload, timeout)

    def get(self, kind, name=None, namespace=True, selector=None):
        return json.loads(self.k("get", kind, *([name] if name else []), *(["-l", selector] if selector else []), "-o", "json", namespace=namespace))

    def apply(self, obj):
        self.k("apply", "-f", "-", payload=json.dumps(obj))

    def require_owner(self):
        obj = self.get("namespace", self.ns, namespace=False)
        if obj["metadata"].get("labels", {}).get("verification.redis.opstreelabs.in/owner") != OWNER:
            raise ValueError("namespace is not owned by this test")

    def operator_stopped(self):
        deployment = json.loads(self.k("-n", "redis-operator-system", "get", "deployment", "redis-operator", "-o", "json", namespace=False))
        pods = json.loads(self.k("-n", "redis-operator-system", "get", "pods", "-l", "name=redis-operator", "-o", "json", namespace=False))
        return deployment["spec"]["replicas"] == 0 and deployment.get("status", {}).get("replicas", 0) == 0 and not pods["items"]

    def cli(self, pod, *args, sentinel=False):
        return self.k("exec", pod, "--", "redis-cli", "--raw", *(["-p", "26379"] if sentinel else []), *args).strip()

    def pods(self, sentinel=False):
        return self.get("pods", selector="app=" + ("redis-sentinel-sentinel" if sentinel else "redis-replication"))["items"]

    def ids(self, sentinel=False):
        return {p["metadata"]["name"]: p["metadata"]["uid"] for p in self.pods(sentinel)}

    def endpoints(self):
        data = self.get("endpointslice", selector="kubernetes.io/service-name=redis-replication-master")
        return sorted(e["targetRef"]["name"] for item in data["items"] for e in (item.get("endpoints") or []) if e.get("conditions", {}).get("ready"))

    def roles(self):
        return {f"redis-replication-{i}": dict(line.split(":", 1) for line in self.cli(f"redis-replication-{i}", "INFO", "replication").splitlines() if ":" in line) for i in range(3)}

    def votes(self):
        result = []
        for i in range(3):
            data = self.cli(f"redis-sentinel-sentinel-{i}", "SENTINEL", "MASTER", GROUP, sentinel=True).splitlines()
            result.append(dict(zip(data[::2], data[1::2])))
        return result

    def healthy(self, observe=None, seconds=240):
        def check():
            if observe:
                observe()
            replication = self.get("redisreplication", "redis-replication")
            sentinel_sts = self.get("statefulset", "redis-sentinel-sentinel")
            generated = bool(sentinel_sts["spec"]["template"]["spec"].get("initContainers"))
            if generated and replication["metadata"].get("annotations", {}).get("redis.opstreelabs.in/sentinel-startup") != "enabled":
                return False
            redis_sts = self.get("statefulset", "redis-replication")
            if generated and not any(e.get("name") == "SENTINEL_BOOTSTRAP_REQUIRED" and e.get("value") == "true" for init in redis_sts["spec"]["template"]["spec"].get("initContainers", []) for e in init.get("env", [])):
                return False
            for sts in (sentinel_sts, redis_sts):
                if sts.get("status", {}).get("observedGeneration", 0) < sts["metadata"].get("generation", 0):
                    return False
                partition = sts["spec"].get("updateStrategy", {}).get("rollingUpdate", {}).get("partition", 0)
                if partition == 0 and sts.get("status", {}).get("updatedReplicas", 0) != 3:
                    return False
            pods = [*self.pods(), *self.pods(True)]
            if len(pods) != 6 or not all(not p["metadata"].get("deletionTimestamp") and any(c["type"] == "Ready" and c["status"] == "True" for c in p["status"].get("conditions", [])) for p in pods):
                return False
            expected = getattr(self.args, "expected_init_image", None)
            if expected and not all(any(init["name"] == "init-config" and init["image"] == expected for init in pod["spec"].get("initContainers", [])) for pod in pods):
                return False
            roles = self.roles()
            masters = [name for name, info in roles.items() if info.get("role") == "master"]
            if len(masters) != 1 or roles[masters[0]].get("connected_slaves") != "2" or any(info.get("master_link_status") != "up" for name, info in roles.items() if name != masters[0]):
                return False
            votes = self.votes()
            primary_ip = next(p["status"]["podIP"] for p in pods if p["metadata"]["name"] == masters[0])
            if self.endpoints() != masters or any(v.get("flags") != "master" or v.get("ip") != primary_ip for v in votes) or len({v["config-epoch"] for v in votes}) != 1:
                return False
            return masters[0]
        return wait(check, "one primary, two direct replicas, three healthy agreeing Sentinels and one endpoint", seconds)

    def marker(self, master, key="local-authority-marker"):
        assert self.cli(master, "SET", key, "synthetic-retained-value") == "OK"
        wait(lambda: all(self.cli(name, "GET", key) == "synthetic-retained-value" for name in self.ids()), "marker replicated")
        for name in self.ids():
            busy = 0
            def save():
                nonlocal busy
                response = self.cli(name, "SAVE")
                if response == "OK":
                    return True
                if response == "ERR Background save already in progress":
                    busy += 1
                    return False
                raise AssertionError("unexpected SAVE response; persistence preflight failed")
            wait(save, "synthetic snapshot saved", 60)
            if busy:
                print(json.dumps({"persistence_preflight": "saved-after-background-child", "pod": name, "busy_responses": busy}), flush=True)

    def assert_marker(self, master, key="local-authority-marker"):
        assert self.cli(master, "GET", key) == "synthetic-retained-value", "synthetic marker lost"

    def replace_sentinels(self, count):
        master = self.healthy()
        self.marker(master)
        original = self.ids()
        before = self.ids(True)
        names = sorted(before)[:count]
        start = time.monotonic()
        self.k("delete", "pod", *names, "--wait=false")
        wait(lambda: all(self.ids(True).get(name) not in (None, before[name]) for name in names), "Sentinel pod UIDs replaced")
        assert self.healthy() == master
        assert self.ids() == original
        self.assert_marker(master)
        return {"replaced": count, "seconds": round(time.monotonic() - start, 2)}

    def request_failover(self):
        reply = self.cli("redis-sentinel-sentinel-0", "SENTINEL", "FAILOVER", GROUP, sentinel=True)
        if reply != "OK":
            code = next((code for code in ("NOGOODSLAVE", "INPROG", "ERR", "NOAUTH", "WRONGPASS") if reply.startswith(code)), "unclassified")
            raise RuntimeError(f"Sentinel refused the forced-failover injection ({code})")

    def failover(self, overlap=False, observe=None):
        old = self.healthy()
        self.marker(old)
        original = self.ids()
        raw = self.k("exec", "redis-sentinel-sentinel-0", "--", "redis-cli", "--json", "-p", "26379", "SENTINEL", "REPLICAS", GROUP)
        known = {p["status"].get("podIP"): p["metadata"]["name"] for p in self.pods()}
        observed = []
        for row in json.loads(raw):
            data = row if isinstance(row, dict) else dict(zip(row[::2], row[1::2]))
            if data.get("ip") in known:
                observed.append({"member": known[data["ip"]], **{key: data.get(key) for key in ("flags", "role-reported", "master-link-status", "slave-priority", "info-refresh", "last-ok-ping-reply", "master-link-down-time")}})
        print(json.dumps({"failover_preflight": "sentinel-0", "replicas": observed}), flush=True)
        self.request_failover()
        def promoted():
            if observe:
                observe()
            return next((name for name, info in self.roles().items() if name != old and info.get("role") == "master"), None)
        new = wait(promoted, "Sentinel promotion", 90)
        if overlap:
            replaced = next(name for name in original if name not in (old, new))
            self.k("delete", "pod", replaced, "--wait=false")
            wait(lambda: self.ids().get(replaced) not in (None, original[replaced]), "replica UID replaced")
        assert self.healthy(observe) == new, "Sentinel election reversed"
        self.assert_marker(new)
        after = self.ids()
        assert all(after[name] == uid for name, uid in original.items() if not overlap or name != replaced)
        return {"old": old, "elected": new, "overlap": overlap}

    def missing_monitors(self, count):
        master = self.healthy()
        self.marker(master)
        original = self.ids()
        for i in range(count):
            assert self.cli(f"redis-sentinel-sentinel-{i}", "SENTINEL", "REMOVE", GROUP, sentinel=True) == "OK"
        # Exercise the status-cleared ordering from the independent review.
        self.k("patch", "redisreplication", "redis-replication", "--subresource=status", "--type=merge", "-p", '{"status":{"masterNode":""}}')
        assert self.healthy() == master
        assert self.ids() == original
        self.assert_marker(master)
        return {"missing_monitors": count, "status_cleared_first": True}

    def missing_statefulset(self):
        master = self.healthy()
        self.marker(master)
        original = self.ids()
        uid = self.get("statefulset", "redis-sentinel-sentinel")["metadata"]["uid"]
        self.k("delete", "statefulset", "redis-sentinel-sentinel", "--cascade=foreground", "--wait=false")
        self.k("patch", "redisreplication", "redis-replication", "--subresource=status", "--type=merge", "-p", '{"status":{"masterNode":""}}')
        wait(lambda: self.get("statefulset", "redis-sentinel-sentinel")["metadata"]["uid"] != uid, "Sentinel StatefulSet recreated")
        assert self.healthy() == master
        assert self.ids() == original
        self.assert_marker(master)
        return {"statefulset_recreated": True, "status_cleared_first": True}

    def combined_loss(self):
        primary = self.healthy()
        self.marker(primary)
        replica = next(name for name in self.ids() if name != primary)
        self.k("delete", "pod", *sorted(self.ids(True)), replica, "--wait=false")
        # A release candidate must either recover this schedule or define and
        # rehearse an explicit safe disaster-recovery boundary. A timeout is
        # intentionally a failing acceptance check, not a successful failover.
        selected = self.healthy()
        self.assert_marker(selected)
        return {"combined_sentinel_and_replica_loss": "pass"}

    def clients(self):
        if self.args.authentication:
            raise ValueError("client probe is for the separate unauthenticated synthetic fixture")
        self.healthy()
        try:
            self.get("pod", "persistent-client-probe")
        except RuntimeError as error:
            if "NotFound" not in str(error):
                raise
        else:
            raise ValueError("client probe already exists; use a fresh fixture rather than reusing completed results")
        duration = self.args.client_duration_seconds
        self.apply({"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "persistent-client-probe", "namespace": self.ns}, "spec": {"restartPolicy": "Never", "containers": [{"name": "probe", "image": self.args.client_image, "imagePullPolicy": "IfNotPresent", "args": [f"--duration={duration}s"]}]}})
        wait(lambda: '"persistent_clients_started":true' in self.k("logs", "persistent-client-probe"), "persistent pools/subscription established")
        time.sleep(10)
        endpoint_views = {}
        observation_start = time.monotonic()
        trace_path = getattr(self.args, "authority_trace", None)
        trace = []
        if trace_path and (not trace_path.resolve().is_relative_to('/var/tmp/daniel/tmp/opencode') or trace_path.exists()):
            raise ValueError("authority trace requires a new approved artifact path")
        def observe():
            view = ",".join(self.endpoints()) or "none"
            elapsed = round(time.monotonic() - observation_start, 2)
            samples = endpoint_views.setdefault(view, {"samples": 0, "first_seconds": elapsed, "last_seconds": elapsed})
            samples["samples"] += 1
            samples["last_seconds"] = elapsed
            if trace_path:
                known = {pod['status'].get('podIP'): pod['metadata']['name'] for pod in self.pods()}
                states = []
                for index, master in enumerate(self.votes()):
                    address = self.cli(f'redis-sentinel-sentinel-{index}', 'SENTINEL', 'GET-MASTER-ADDR-BY-NAME', GROUP, sentinel=True).splitlines()
                    states.append({'sentinel': index, 'master_member': known.get(master.get('ip'), 'not-current'),
                                   'epoch': master.get('config-epoch'), 'flags': master.get('flags'),
                                   'failover_state': master.get('failover-state'),
                                   'client_member': known.get(address[0], 'not-current') if address else 'none'})
                trace.append({'seconds': elapsed, 'endpoints': view, 'reports': states})
                trace_path.write_text(json.dumps(trace, indent=2))
        result = self.failover(observe=observe)
        def finished():
            observe()
            return self.get("pod", "persistent-client-probe")["status"]["phase"] in ("Succeeded", "Failed")
        wait(finished, "persistent-client probe finished", duration + 30)
        lines = self.k("logs", "persistent-client-probe").splitlines()
        summaries = [json.loads(line) for line in lines if line.startswith('{"counts"') or (line.startswith("{") and '"last_success_seconds"' in line)]
        assert len(summaries) == 1, "missing client result"
        summary = summaries[0]
        placement = {pod["metadata"]["name"]: pod["spec"]["nodeName"] for pod in self.pods()}
        placement["probe"] = self.get("pod", "persistent-client-probe")["spec"]["nodeName"]
        print(json.dumps({"client_probe": summary, "post_failover_endpoint_observations": endpoint_views, "placement": placement}), flush=True)
        assert summary["passed"], "persistent clients did not recover"
        return {"failover": result, "clients": summary}

    def delayed(self):
        old = self.healthy()
        self.marker(old)
        replicas = [name for name in self.ids() if name != old]
        held, candidate = sorted(replicas)
        self.cli(held, "CONFIG", "SET", "replica-priority", "0")
        pods = {p["metadata"]["name"]: p for p in self.pods()}
        rules = []
        try:
            for target in (old, held):
                for sentinel in self.pods(True):
                    node = pods[target]["spec"]["nodeName"]
                    args = ["FORWARD", "-s", sentinel["status"]["podIP"], "-d", pods[target]["status"]["podIP"], "-p", "tcp", "--dport", "6379", "-j", "DROP"]
                    command(["docker", "exec", node, "iptables", "-I", *args])
                    rules.append((node, args))
            self.request_failover()
            wait(lambda: sum(v.get("role") == "master" for v in self.roles().values()) > 1, "required two-primary race window", 90)
            self.k("patch", "redisreplication", "redis-replication", "--subresource=status", "--type=merge", "-p", json.dumps({"status": {"connectionInfo": {"host": "local-authority-test.invalid"}}}))
            time.sleep(15)
            assert self.roles()[candidate]["role"] == "master", "newly elected primary was demoted during race"
        finally:
            for node, args in reversed(rules):
                command(["docker", "exec", node, "iptables", "-D", *args])
            self.cli(held, "CONFIG", "SET", "replica-priority", "100")
        assert self.healthy() == candidate
        self.assert_marker(candidate)
        return {"old": old, "elected": candidate}

    def install_topology(self):
        try:
            self.get("namespace", self.ns, namespace=False)
        except RuntimeError as error:
            if "NotFound" not in str(error):
                raise
        else:
            raise ValueError("refusing to initialize an existing namespace")
        self.apply({"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": self.ns, "labels": {"verification.redis.opstreelabs.in/owner": OWNER}}})
        if self.args.authentication:
            # Disposable, randomly generated credentials travel only in stdin
            # manifests and SecretKeyRefs, never argv, receipts or source fixtures.
            for name in ("synthetic-redis-auth", "synthetic-sentinel-auth"):
                self.apply({"apiVersion": "v1", "kind": "Secret", "metadata": {"name": name, "namespace": self.ns}, "type": "Opaque", "stringData": {"password": secrets.token_urlsafe(32)}})
        for kind, name, image in (("RedisReplication", "redis-replication", "redis"), ("RedisSentinel", "redis-sentinel", "redis-sentinel")):
            spec = {"clusterSize": 3, "podSecurityContext": {"runAsUser": 1000, "fsGroup": 1000}, "kubernetesConfig": {"image": f"quay.io/opstree/{image}:v8.4.0", "imagePullPolicy": "IfNotPresent", "resources": {"requests": {"cpu": "50m", "memory": "64Mi"}, "limits": {"cpu": "500m", "memory": "256Mi"}}}}
            if self.args.authentication:
                auth = "synthetic-redis-auth" if kind == "RedisReplication" else "synthetic-sentinel-auth"
                spec["kubernetesConfig"]["redisSecret"] = {"name": auth, "key": "password"}
                spec["env"] = [{"name": "REDISCLI_AUTH", "valueFrom": {"secretKeyRef": {"name": auth, "key": "password"}}}]
            if kind == "RedisReplication":
                spec["storage"] = {"volumeClaimTemplate": {"spec": {"storageClassName": "standard", "accessModes": ["ReadWriteOnce"], "resources": {"requests": {"storage": "1Gi"}}}}}
            else:
                spec["redisSentinelConfig"] = {"redisReplicationName": "redis-replication", "masterGroupName": GROUP, "quorum": "2", "downAfterMilliseconds": "10000", "failoverTimeout": "60000", "parallelSyncs": "1"}
                if not self.args.ephemeral_sentinels:
                    spec["storage"]={"volumeClaimTemplate":{"spec":{"storageClassName":"standard","accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"1Gi"}}}}}
                if self.args.authentication:
                    spec["redisSentinelConfig"]["redisReplicationPassword"] = {"secretKeyRef": {"name": "synthetic-redis-auth", "key": "password"}}
            self.apply({"apiVersion": "redis.redis.opstreelabs.in/v1beta2", "kind": kind, "metadata": {"name": name, "namespace": self.ns}, "spec": spec})
        return {"primary": self.healthy(seconds=getattr(self.args, "setup_timeout_seconds", 240))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kubeconfig", type=pathlib.Path, required=True)
    parser.add_argument("--cluster", required=True)
    parser.add_argument("--namespace", default="authority-test-main")
    parser.add_argument("--kubectl", default="kubectl")
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--authentication", action="store_true")
    parser.add_argument("--ephemeral-sentinels",action="store_true",help="legacy baseline only; current standalone authority requires durable state")
    parser.add_argument("--client-image", default="redis-authority-client-probe:local")
    parser.add_argument("--client-duration-seconds", type=int, choices=(90, 180, 300), default=90)
    parser.add_argument("--expected-init-image", help="require every actual Redis/Sentinel pod to use this exact init image")
    parser.add_argument("--authority-trace", type=pathlib.Path, help="new approved local file for selected noncredential authority fields during a client check")
    parser.add_argument("mode", choices=["setup", "health", "regressions", "recovery", "clients", "combined-loss"])
    args = parser.parse_args()
    cluster = Cluster(args)
    if args.mode == "setup":
        print(json.dumps({"setup": cluster.install_topology()}), flush=True)
        return
    cluster.require_owner()
    if args.mode == "health":
        print(json.dumps({"primary": cluster.healthy()}), flush=True)
        return
    if args.mode == "recovery":
        # Removing an established monitor destroys per-master vote history;
        # automatic reseeding is no longer a valid recovery acceptance case.
        for name, test in (("missing-statefulset", cluster.missing_statefulset), ("authenticated-failover" if args.authentication else "failover", cluster.failover)):
            print(json.dumps({"test": name, "result": test(), "passed": True}), flush=True)
        return
    if args.mode == "clients":
        print(json.dumps({"test": "persistent-clients", "result": cluster.clients(), "passed": True}), flush=True)
        return
    if args.mode == "combined-loss":
        print(json.dumps(cluster.combined_loss()),flush=True)
        return
    for round_number in range(args.rounds):
        for name, test in (("delayed-election", cluster.delayed), ("ordinary-failover", cluster.failover), ("election-plus-replacement", lambda: cluster.failover(True)), ("two-sentinel-replacements", lambda: cluster.replace_sentinels(2)), ("all-sentinel-replacements", lambda: cluster.replace_sentinels(3))):
            print(json.dumps({"round": round_number + 1, "test": name, "result": test(), "passed": True}), flush=True)


if __name__ == "__main__":
    main()
