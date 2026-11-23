#!/usr/bin/env python3
"""Guarded kind test: fail over after init, before replacement Redis starts."""
import argparse
import datetime
import json
import pathlib
import time

from verify import Cluster, GROUP, command, wait


class ReplacementCluster(Cluster):
    def install_topology(self):
        # Preserve the ordinary fixture and its storage/authentication contract;
        # only the main Redis image and initial liveness grace differ.
        original_apply = self.apply

        def apply(obj):
            if obj.get("kind") == "RedisReplication":
                obj["spec"]["kubernetesConfig"]["image"] = self.args.delay_image
                obj["spec"]["livenessProbe"] = {"initialDelaySeconds": 180, "periodSeconds": 15, "timeoutSeconds": 5}
                if self.args.generic_ephemeral_claim:
                    obj["spec"]["storage"]["volumeMount"] = {
                        "volume": [{"name": "ephemeral-state", "ephemeral": {"volumeClaimTemplate": {"spec": {"storageClassName": "standard", "accessModes": ["ReadWriteOnce"], "resources": {"requests": {"storage": "64Mi"}}}}}}],
                        "mountPath": [{"name": "ephemeral-state", "mountPath": "/scratch"}],
                    }
            return original_apply(obj)

        self.apply = apply
        try:
            return super().install_topology()
        finally:
            self.apply = original_apply

    def replicas(self, index):
        result = json.loads(self.k("exec", f"redis-sentinel-sentinel-{index}", "--", "redis-cli", "--json", "-p", "26379", "SENTINEL", "REPLICAS", GROUP))
        return [row if isinstance(row, dict) else dict(zip(row[::2], row[1::2])) for row in result]

    def voters(self):
        return [self.cli(f"redis-sentinel-sentinel-{i}", "SENTINEL", "MYID", sentinel=True) for i in range(3)]

    def claims(self):
        return {p["metadata"]["name"]: p["metadata"]["uid"] for p in self.get("pvc")["items"]}

    def restart_operator(self):
        def operator_pods():
            return json.loads(self.k("-n", "redis-operator-system", "get", "pods", "-l", "name=redis-operator", "-o", "json", namespace=False))["items"]

        old = operator_pods()
        assert len(old) == 1, "requires one disposable operator process"
        uid = old[0]["metadata"]["uid"]
        self.k("-n", "redis-operator-system", "delete", "pod", old[0]["metadata"]["name"], "--wait=false", namespace=False)
        wait(lambda: any(p["metadata"]["uid"] != uid and any(c["type"] == "Ready" and c["status"] == "True" for c in p["status"].get("conditions", [])) for p in operator_pods()), "operator process replaced", 120)

    def block_operator_voter(self):
        operator = json.loads(self.k("-n", "redis-operator-system", "get", "pods", "-l", "name=redis-operator", "-o", "json", namespace=False))["items"]
        assert len(operator) == 1 and operator[0]["status"].get("podIP"), "requires one disposable operator"
        voter = self.pods(True)[-1]
        node = voter["spec"]["nodeName"]
        assert node.startswith(self.args.cluster + "-")
        rule = ["FORWARD", "-s", operator[0]["status"]["podIP"], "-d", voter["status"]["podIP"], "-p", "tcp", "--dport", "26379", "-j", "DROP"]
        command(["docker", "exec", node, "iptables", "-I", *rule])
        return node, rule

    def deletion_hold(self, name, uid, install):
        marker = "verification.redis.opstreelabs.in/replacement-test"
        pod = self.get("pod", name)
        assert pod["metadata"]["uid"] == uid, "refusing to change a successor's finalizers"
        finalizers = list(pod["metadata"].get("finalizers", []))
        if install and marker not in finalizers:
            finalizers.append(marker)
        elif not install:
            finalizers = [value for value in finalizers if value != marker]
        patch = [{"op": "test", "path": "/metadata/uid", "value": uid},
                 {"op": "test", "path": "/metadata/resourceVersion", "value": pod["metadata"]["resourceVersion"]},
                 {"op": "add", "path": "/metadata/finalizers", "value": finalizers}]
        self.k("patch", "pod", name, "--type=json", "-p", json.dumps(patch))

    def restart_observation(self, target, uid):
        self.deletion_hold(target, uid, True)
        try:
            def observation():
                rr = self.get("redisreplication", "redis-replication")
                ledger = json.loads(rr["metadata"].get("annotations", {}).get("redis.opstreelabs.in/sentinel-replacement-recovery", "{}"))
                assert ledger.get("attempts", 0) == 0, "missed the unreserved observation window"
                return ledger.get("observation")

            first = wait(observation, "first controller observation", 180)
            assert first.get("controller"), "fixture requires controller-incarnation fencing"
            self.restart_operator()

            def fresh_observation():
                current = observation()
                return current if current and current.get("controller") != first["controller"] else None

            fresh = wait(fresh_observation, "new process establishes its own observation", 180)

            def deletion_requested():
                pod = self.get("pod", target)
                assert pod["metadata"]["uid"] == uid
                return pod["metadata"].get("deletionTimestamp")

            deleted_at = wait(deletion_requested, "recovery deletion requested after fresh interval", 180)
            since = datetime.datetime.fromisoformat(fresh["since"].replace("Z", "+00:00"))
            deleted = datetime.datetime.fromisoformat(deleted_at.replace("Z", "+00:00"))
            elapsed = (deleted - since).total_seconds()
            # API metav1.Time serialization can truncate the request to seconds.
            assert elapsed >= 29, "controller restart reused the old interval"
            print(json.dumps({"operator_restart_fresh_interval_seconds": elapsed, "attempts_before_restart": 0}), flush=True)
        finally:
            self.deletion_hold(target, uid, False)

    def run_replacement(self):
        self.require_owner()
        old = self.healthy()
        self.marker(old)
        original = self.ids()
        sentinel_ids, voters, claims = self.ids(True), self.voters(), self.claims()
        if self.args.exhausted_budget:
            rr = self.get("redisreplication", "redis-replication")
            state = {"version": 1, "owner": rr["metadata"]["uid"], "attempts": 2}
            self.k("annotate", "redisreplication", "redis-replication", "redis.opstreelabs.in/sentinel-replacement-recovery=" + json.dumps(state, separators=(",", ":")), "--overwrite")
        target = sorted(name for name in original if name != old)[-1]
        elected = next(name for name in original if name not in (old, target))
        old_ip = self.get("pod", old)["status"]["podIP"]
        previous_target_ip = self.get("pod", target)["status"]["podIP"]
        # Only these fixture-owned marker files may be removed; never touch data.
        self.k("exec", target, "--", "rm", "-f", "/data/.authority-delay-release", "/data/.authority-delay-consumed")
        self.k("exec", target, "--", "touch", "/data/.authority-delay-arm")
        self.k("delete", "pod", target, "--wait=false")
        wait(lambda: self.ids().get(target) not in (None, original[target]), "replacement UID created")
        delayed_uid = self.ids()[target]

        def initialized_and_held():
            pod = self.get("pod", target)
            initialized = any(s["name"] == "init-config" and s.get("state", {}).get("terminated", {}).get("exitCode") == 0 for s in pod["status"].get("initContainerStatuses", []))
            if not initialized:
                return False
            self.k("exec", target, "--", "test", "-f", "/data/.authority-delay-consumed")
            return True

        wait(initialized_and_held, "protected init finished and main Redis held", 180)
        # The unavailable old address must not be selected as an election
        # candidate while the new address still has no Redis process.
        wait(lambda: all(any(row.get("ip") == previous_target_ip and "s_down" in row.get("flags", "") for row in self.replicas(i)) for i in range(3)), "Sentinels observe removed replica unavailable", 60)
        self.request_failover()

        def elected_and_reconfigured():
            current = self.roles_without(target)
            return current[elected].get("role") == "master" and current[old].get("role") == "slave" and current[old].get("master_host") == self.get("pod", elected)["status"]["podIP"] and current[old].get("master_link_status") == "up"

        wait(elected_and_reconfigured, "Sentinel promotes survivor and demotes old primary", 90)
        elected_ip = self.get("pod", elected)["status"]["podIP"]
        wait(lambda: all(v.get("ip") == elected_ip and v.get("flags") == "master" for v in self.votes()), "all retained voters agree after promotion", 90)
        epoch = self.votes()[0]["config-epoch"]
        self.k("exec", target, "--", "touch", "/data/.authority-delay-release")

        def cascading():
            info = self.roles()[target]
            return info.get("role") == "slave" and info.get("master_host") == old_ip and info.get("master_link_status") == "up"

        wait(cascading, "required cascading replacement race", 60)
        target_ip = self.get("pod", target)["status"]["podIP"]
        assert all(not any(row.get("ip") == target_ip for row in self.replicas(i)) for i in range(3)), "race did not leave the replacement undiscovered"
        print(json.dumps({"race_reproduced": True, "old_primary": old, "elected": elected, "target": target, "delayed_uid": delayed_uid}), flush=True)
        if self.args.restart_during_observation:
            self.restart_observation(target, delayed_uid)
        preserved_claims = claims
        if self.args.generic_ephemeral_claim:
            preserved_claims = self.claims()
            auxiliary = target + "-ephemeral-state"
            assert preserved_claims[auxiliary] != claims[auxiliary], "fixture deletion did not replace the Pod-owned auxiliary claim"
            assert all(preserved_claims[name] == uid for name, uid in claims.items() if name != auxiliary), "fixture changed a retained or healthy-member claim"
            self.k("exec", target, "--", "touch", "/scratch/.authority-claim-marker")
        if self.args.exhausted_budget or self.args.unavailable_voter or self.args.generic_ephemeral_claim:
            blocked = self.block_operator_voter() if self.args.unavailable_voter else None
            try:
                if self.args.exhausted_budget:
                    self.restart_operator()
                deadline = time.monotonic() + 150
                while time.monotonic() < deadline:
                    assert self.ids().get(target) == delayed_uid, "negative recovery gate deleted the replacement"
                    assert self.roles()[elected]["role"] == "master", "negative recovery gate lost the serving primary"
                    time.sleep(5)
                if blocked:
                    node, rule = blocked
                    counters = command(["docker", "exec", node, "iptables", "-nvx", "-L", "FORWARD"])
                    dropped = [int(line.split()[0]) for line in counters.splitlines() if "DROP" in line and rule[2] in line.split() and rule[4] in line.split() and "dpt:26379" in line]
                    assert dropped and max(dropped) > 0, "unavailable-voter injection did not intercept operator traffic"
                    print(json.dumps({"blocked_operator_voter_packets": max(dropped)}), flush=True)
            finally:
                if blocked:
                    node, rule = blocked
                    command(["docker", "exec", node, "iptables", "-D", *rule])
            assert self.claims() == preserved_claims and self.ids(True) == sentinel_ids and self.voters() == voters
            if self.args.exhausted_budget:
                ledger = json.loads(self.get("redisreplication", "redis-replication")["metadata"]["annotations"]["redis.opstreelabs.in/sentinel-replacement-recovery"])
                assert ledger["attempts"] == 2, "operator restart replenished the recovery budget"
                assert self.ids()[target] == delayed_uid
                logs = self.k("-n", "redis-operator-system", "logs", "deployment/redis-operator", "--since=4m", namespace=False)
                exhausted = False
                for line in logs.splitlines():
                    try:
                        entry = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if entry.get("namespace") == self.ns and entry.get("msg", "").startswith("Automatic replacement recovery budget exhausted"):
                        exhausted = True
                assert exhausted, "operator never reached the exhausted-budget gate"
                print(json.dumps({"passed": True, "negative": "exhausted-budget-across-operator-restart", "attempts": 2, "delayed_uid_retained": True, "claims_retained": True, "voters_retained": True}), flush=True)
                return
            if self.args.generic_ephemeral_claim:
                rr = self.get("redisreplication", "redis-replication")
                ledger = json.loads(rr["metadata"].get("annotations", {}).get("redis.opstreelabs.in/sentinel-replacement-recovery", "{}"))
                assert ledger.get("attempts", 0) == 0, "generic ephemeral volume consumed recovery budget"
                self.k("exec", target, "--", "test", "-f", "/scratch/.authority-claim-marker")
                print(json.dumps({"passed": True, "negative": "generic-ephemeral-claim", "attempts": 0, "delayed_uid_retained": True, "all_current_claims_retained": True, "voters_retained": True}), flush=True)
                return
            print(json.dumps({"negative": "unavailable-voter", "delayed_uid_retained": True, "gate_released": True}), flush=True)
        started = time.monotonic()
        wait(lambda: self.ids().get(target) not in (None, delayed_uid), "bounded operator recovery recreates stranded UID", 300)
        assert self.healthy() == elected
        elapsed = round(time.monotonic() - started, 3)
        after = self.ids()
        assert all(after[name] == original[name] for name in original if name != target), "recovery changed a healthy member"
        assert self.ids(True) == sentinel_ids and self.voters() == voters, "recovery changed a retained voter"
        assert self.claims() == claims, "recovery changed a retained claim"
        assert all(v["config-epoch"] == epoch for v in self.votes()), "recovery triggered another election"
        pod = self.get("pod", target)
        assert all(s.get("restartCount", 0) == 0 for s in pod["status"].get("containerStatuses", [])), "recovery used container restarts instead of protected initialization"
        self.assert_marker(elected)
        ledger = json.loads(self.get("redisreplication", "redis-replication")["metadata"]["annotations"]["redis.opstreelabs.in/sentinel-replacement-recovery"])
        assert ledger["attempts"] == 1, "recovery consumed the wrong budget"
        result = {"passed": True, "authentication": self.args.authentication, "seconds_after_cascading": elapsed, "target": target, "delayed_uid": delayed_uid, "recovered_uid": after[target], "attempts": ledger["attempts"], "claims_retained": True, "voters_retained": True, "epoch_unchanged_after_promotion": True}
        print(json.dumps(result), flush=True)
        return result

    def roles_without(self, excluded):
        return {name: dict(line.split(":", 1) for line in self.cli(name, "INFO", "replication").splitlines() if ":" in line) for name in self.ids() if name != excluded}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kubeconfig", type=pathlib.Path, required=True)
    parser.add_argument("--cluster", required=True)
    parser.add_argument("--namespace", default="authority-test-replacement")
    parser.add_argument("--kubectl", default="kubectl")
    parser.add_argument("--authentication", action="store_true")
    parser.add_argument("--delay-image", default="redis-authority-replacement-delay:local")
    parser.add_argument("--expected-init-image")
    parser.add_argument("--setup-timeout-seconds", type=int, default=480)
    negative = parser.add_mutually_exclusive_group()
    negative.add_argument("--exhausted-budget", action="store_true")
    negative.add_argument("--unavailable-voter", action="store_true")
    negative.add_argument("--generic-ephemeral-claim", action="store_true")
    negative.add_argument("--restart-during-observation", action="store_true")
    parser.add_argument("mode", choices=("setup", "recovery"))
    args = parser.parse_args()
    args.ephemeral_sentinels = False
    cluster = ReplacementCluster(args)
    if args.mode == "setup":
        print(json.dumps(cluster.install_topology()), flush=True)
    else:
        cluster.run_replacement()


if __name__ == "__main__":
    main()
