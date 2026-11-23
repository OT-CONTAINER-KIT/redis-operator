#!/usr/bin/env python3
"""Check same-voter, same-epoch vote retention across a disposable Pod replacement."""
import argparse
import json
import pathlib
import time

import verify


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--kubeconfig',type=pathlib.Path,required=True)
    p.add_argument('--cluster',required=True)
    p.add_argument('--namespace',default='authority-test-discovery')
    p.add_argument('--kubectl',default='kubectl')
    p.add_argument('--allow-ephemeral',action='store_true',help='only for reproducing the historical broken candidate')
    p.add_argument('--loss-mode',choices=('pod','monitor'),default='pod')
    p.add_argument('--expect-blocked-monitor-loss',action='store_true')
    p.add_argument('--expected-init-image')
    args=p.parse_args()
    c=verify.Cluster(args);c.require_owner()
    master=c.healthy()
    if args.expect_blocked_monitor_loss and args.loss_mode!='monitor':p.error('blocked-loss expectation requires monitor loss')
    original_redis=c.ids()
    c.marker(master)
    ip=c.get('pod',master)['status']['podIP']
    pods=[f'redis-sentinel-sentinel-{i}' for i in range(3)]
    target=c.get('pod',pods[1])
    claims=[v['persistentVolumeClaim']['claimName'] for v in target['spec']['volumes'] if v['name']=='sentinel-state' and 'persistentVolumeClaim' in v]
    if not claims and not args.allow_ephemeral:raise RuntimeError('vote-retention acceptance requires a durable state claim')
    claim_uid=c.get('pvc',claims[0])['metadata']['uid'] if claims else None
    identities=[c.cli(name,'SENTINEL','MYID',sentinel=True) for name in pods]
    def request(pod,candidate,epoch):
        raw=c.k('exec',pod,'--','redis-cli','--json','-p','26379','SENTINEL','IS-MASTER-DOWN-BY-ADDR',ip,'6379',str(epoch),candidate)
        data=json.loads(raw)
        return data[1],int(data[2])

    # Epoch zero cannot advance a vote. A real candidate (unlike '*') makes
    # Redis return the previously recorded leader epoch for repeatable tests.
    epoch=max([int(v['config-epoch']) for v in c.votes()]+[request(pod,identities[0],0)[1] for pod in pods])+1000

    def vote(pod,candidate):
        return request(pod,candidate,epoch)

    a=identities[0];b=identities[2]
    first_self=vote(pods[0],a)
    first_shared=vote(pods[1],a)
    if first_self!=(a,epoch) or first_shared!=(a,epoch):
        raise RuntimeError('test setup did not obtain a fresh vote epoch; retention was not evaluated')
    assert vote(pods[1],b)==(a,epoch),'voter changed its vote even before replacement'
    second_self=vote(pods[2],b)
    assert second_self==(b,epoch)
    uid=c.ids(True)[pods[1]]
    if args.loss_mode=='pod':
        c.k('delete','pod',pods[1],'--wait=false')
        verify.wait(lambda:c.ids(True).get(pods[1]) not in (None,uid),'voter Pod replaced')
    else:
        assert c.cli(pods[1],'SENTINEL','REMOVE',verify.GROUP,sentinel=True)=='OK'
        if args.expect_blocked_monitor_loss:
            c.k('patch','redisreplication','redis-replication','--subresource=status','--type=merge','-p','{"status":{"masterNode":"","connectionInfo":null}}')
            def blocked():
                return 'No such master' in c.cli(pods[1],'SENTINEL','MASTER',verify.GROUP,sentinel=True) and not c.endpoints()
            verify.wait(blocked,'known monitor loss rejected without recreating its voting state',60)
            for _ in range(5):
                time.sleep(3)
                assert blocked(),'lost monitor was reinitialized or routing reopened'
                assert vote(pods[1],b)!=(b,epoch),'lost-history voter granted a second candidate in the old epoch'
            assert c.ids()==original_redis
            assert c.cli(pods[1],'SENTINEL','MYID',sentinel=True)==identities[1]
            assert c.get('pvc',claims[0])['metadata']['uid']==claim_uid
            c.assert_marker(master)
            print(json.dumps({'loss_mode':'monitor','known_history_loss_fail_closed':True,'monitor_not_reinitialized':True,'same_voter_identity':True,'same_state_claim':True,'second_candidate_not_granted':True,'redis_pod_uids_retained':True,'marker_retained':True,'requires_explicit_generation_recovery':True,'passed':True,'scope':'vote responses and fail-closed routing only; no failover or promotion was requested'}),flush=True)
            return
    c.healthy()
    after_id=c.cli(pods[1],'SENTINEL','MYID',sentinel=True)
    after_vote=vote(pods[1],b)
    double_vote=after_id==identities[1] and after_vote==(b,epoch)
    same_claim=claim_uid is not None and c.get('pvc',claims[0])['metadata']['uid']==claim_uid
    passed=not double_vote and after_id==identities[1] and after_vote[1]==epoch and same_claim
    print(json.dumps({'loss_mode':args.loss_mode,'same_voter_identity':after_id==identities[1],'same_state_claim':same_claim,'vote_epoch_retained':after_vote[1]==epoch,'first_majority_votes':2,'second_candidate_self_vote':True,'same_epoch_vote_changed_after_replacement':double_vote,'passed':passed,'scope':'vote responses only; no failover or promotion was requested'}),flush=True)
    if not passed:raise SystemExit(1)


if __name__=='__main__':main()
