#!/usr/bin/env python3
"""Finite public Pintle site and genuine Linux binary acquisition."""
import argparse,gzip,hashlib,html,io,json,os,pathlib,shutil,subprocess,tarfile
ROOT=pathlib.Path(__file__).resolve().parent.parent
p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--go',default='go');p.add_argument('--base',default='/pintle');p.add_argument('--out',required=True);a=p.parse_args()
base=a.base.rstrip('/')
if base not in ['', '/pintle']:raise ValueError('Unexpected public base')
out=pathlib.Path(a.out).resolve()
if out.exists() or out==ROOT or ROOT in out.parents:raise ValueError('Select a fresh external task output')
binary=pathlib.Path(a.binary).resolve();data=binary.read_bytes()
if not data.startswith(b'\x7fELF') or int.from_bytes(data[18:20],'little')!=62:raise ValueError('Expected genuine Linux x64 ELF binary')
info=subprocess.check_output([a.go,'version','-m',str(binary)],text=True)
if 'github.com/aylith-labs/pintle' not in info or 'CGO_ENABLED=0' not in info:raise ValueError('Unexpected binary build identity')
def stream_json(s):
 dec=json.JSONDecoder();i=0
 while i<len(s):
  while i<len(s) and s[i].isspace():i+=1
  if i>=len(s):break
  x,n=dec.raw_decode(s,i);yield x;i=n
modules={}
for x in stream_json(subprocess.check_output([a.go,'list','-deps','-json','./cmd/pintle'],cwd=ROOT,text=True)):
 m=x.get('Module')
 if m and not m.get('Main'):modules[m['Path']]=m
files={'pintle':(data,0o755),'LICENSE':((ROOT/'LICENSE').read_bytes(),0o644),'SETUP.md':((ROOT/'website/SETUP.md').read_bytes(),0o644)}
notice=[]
for name,m in sorted(modules.items()):
 directory=pathlib.Path(m['Dir']);matches=[p for p in directory.iterdir() if p.is_file() and p.name.upper().startswith(('LICENSE','NOTICE','COPYING','COPYRIGHT'))]
 if not matches:raise ValueError('Missing original license materials for '+name)
 for f in sorted(matches):
  n='notices/'+name.replace('/','__')+'@'+m['Version']+'/'+f.name;files[n]=(f.read_bytes(),0o644);notice.append({'module':name,'version':m['Version'],'file':n,'sha256':hashlib.sha256(f.read_bytes()).hexdigest()})
goroot=pathlib.Path(subprocess.check_output([a.go,'env','GOROOT'],text=True).strip());files['notices/Go-LICENSE']=((goroot/'LICENSE').read_bytes(),0o644)
# Preserve original embedded UI dependency materials conservatively from the actual lock-installed closure.
for directory in sorted((ROOT/'ui/node_modules').glob('*')):
 candidates=list(directory.iterdir()) if directory.name.startswith('@') and directory.is_dir() else [directory]
 for package in candidates:
  if not (package/'package.json').is_file():continue
  meta=json.loads((package/'package.json').read_text());name=meta.get('name');version=meta.get('version')
  if not isinstance(name,str) or not isinstance(version,str):raise ValueError('Invalid installed package identity')
  for f in sorted(package.iterdir()):
   if f.is_file() and f.name.upper().startswith(('LICENSE','NOTICE','COPYING','COPYRIGHT')):
    n='notices/ui/'+name.replace('/','__')+'@'+version+'/'+f.name;files[n]=(f.read_bytes(),0o644);notice.append({'module':name,'version':version,'file':n,'sha256':hashlib.sha256(f.read_bytes()).hexdigest()})
source={'repository':'https://github.com/aylith-labs/pintle','canonicalCommit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'preparedWorkingTree':bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True).strip()),'binarySHA256':hashlib.sha256(data).hexdigest(),'goBuildInfo':info,'licenseInventory':notice}
files['SOURCE.json']=(json.dumps(source,indent=2).encode(),0o644)
files['MANIFEST.json']=(json.dumps({n:{'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b),'mode':oct(m)} for n,(b,m) in files.items()},indent=2).encode(),0o644)
out.mkdir(parents=True);downloads=out/'downloads';downloads.mkdir();archive=downloads/'pintle-linux-x64.tar.gz'
with archive.open('wb') as raw,gzip.GzipFile(filename='',mode='wb',fileobj=raw,mtime=0) as gz,tarfile.open(fileobj=gz,mode='w') as tar:
 for name,(body,mode) in sorted(files.items()):
  t=tarfile.TarInfo('pintle-linux-x64/'+name);t.size=len(body);t.mode=mode;t.mtime=0;tar.addfile(t,io.BytesIO(body))
sha=hashlib.sha256(archive.read_bytes()).hexdigest();(downloads/(archive.name+'.sha256')).write_text(sha+'  '+archive.name+'\n')
assets=out/'assets';assets.mkdir()
for n in ['site.css','site.js']:shutil.copy(ROOT/'website'/n,assets/n)
s=(ROOT/'website/index.html').read_text();origin='https://pintle.aylith.com' if not base else 'https://aylith-labs.github.io'
for k,v in {'BASE':base,'ORIGIN':origin,'SHA':sha}.items():s=s.replace('@@'+k+'@@',html.escape(v,quote=True))
for n in ['site.css','site.js']:s=s.replace('/assets/'+n,'/assets/'+n+'?v='+hashlib.sha256((assets/n).read_bytes()).hexdigest()[:12])
if '@@' in s:raise ValueError('Unresolved public template')
(out/'index.html').write_text(s);(out/'home').mkdir();(out/'home/index.html').write_text(s);(out/'.nojekyll').write_text('');(out/'acquisition.json').write_text(json.dumps({'archive':archive.name,'sha256':sha,'bytes':archive.stat().st_size,'source':source['canonicalCommit'],'preparedWorkingTree':source['preparedWorkingTree'],'binarySHA256':source['binarySHA256']},indent=2))
print(json.dumps({'output':str(out),'archiveSHA256':sha,'members':len(files),'noticeFiles':len(notice)}))
