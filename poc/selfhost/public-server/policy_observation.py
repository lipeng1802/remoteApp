"""Read-only private operator snapshot; never authorizes a product session."""
import base64
import json

SCRIPT="""import subprocess,json
p=subprocess.Popen(['/opt/remoteapp-poc/headscale','-c','/etc/remoteapp-poc/config.yaml','-o','json','policy','get'],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
out,err=p.communicate()
v=json.loads(out) if p.returncode==0 and len(out)<=1048576 else None
v=json.loads(v) if isinstance(v,basestring) else v
rows=v.get('acls') if isinstance(v,dict) else None
print(json.dumps({'query_ok':isinstance(rows,list),'payload_rule_count':sum(1 for a in (rows or []) for d in a.get('dst',[]) if d.endswith(':47476')),'proof_rule_count':sum(1 for a in (rows or []) for d in a.get('dst',[]) if d.endswith(':47477'))}))
"""


def checked(value):
    if type(value) is not dict or set(value)!={'query_ok','payload_rule_count','proof_rule_count'} or type(value['query_ok']) is not bool:
        raise RuntimeError('policy_observation_rejected')
    if any(type(value[k]) is not int or not 0<=value[k]<=256 for k in ('payload_rule_count','proof_rule_count')):
        raise RuntimeError('policy_observation_rejected')
    return value


def observe(work):
    audit=work/'provision-policy-observation.json'
    if audit.exists() or audit.is_symlink():raise FileExistsError("policy observation already recorded")
    import run_supervised_activity as r
    value={'query_ok':False,'payload_rule_count':0,'proof_rule_count':0}
    command="python -c \"import base64;exec(base64.b64decode('"+base64.b64encode(SCRIPT.encode()).decode()+"'))\""
    try:value=checked(json.loads(r.command(['ssh',*r.OPTIONS,r.SERVER,command],15)))
    except Exception:pass
    with audit.open('x') as f:
        json.dump({'version':1,**value},f,sort_keys=True)
