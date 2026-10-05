#!/usr/bin/env python3
import argparse,hashlib,html.parser,json,pathlib,urllib.parse
p=argparse.ArgumentParser();p.add_argument('output');a=p.parse_args();root=pathlib.Path(a.output).resolve();meta=json.loads((root/'acquisition.json').read_text());assert hashlib.sha256((root/'downloads'/meta['archive']).read_bytes()).hexdigest()==meta['sha256'];targets=[]
class Page(html.parser.HTMLParser):
 def handle_starttag(self,t,attrs):
  for k,v in attrs:
   if k not in ('href','src') or not v or v.startswith(('#','https:','http:','data:')):continue
   path=urllib.parse.urlsplit(v).path
   if path.startswith('/pintle/'):path=path[len('/pintle'):]
   target=root/path.lstrip('/')
   if path.endswith('/'):target=target/'index.html'
   if not target.is_file() or root not in target.resolve().parents:raise ValueError('Missing actual public target '+v)
   targets.append(v)
for name in ['index.html','home/index.html']:
 text=(root/name).read_text()
 for requirement in ['Get Pintle','Download Linux x64','mkcert','Static-only','Docker','SNI','No service starts','checksum']:
  if requirement not in text:raise ValueError('Missing full homepage content '+requirement)
 Page().feed(text)
assert (root/'index.html').read_bytes()==(root/'home/index.html').read_bytes();print(json.dumps({'fullPages':2,'actualLocalTargets':len(targets),'actualArchiveSHA':meta['sha256']}))
