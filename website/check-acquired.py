#!/usr/bin/env python3
"""Actual acquired Pintle public CLI, HTTPS proxy, dashboard and reload acceptance."""
import argparse,hashlib,http.client,http.server,json,pathlib,socket,ssl,subprocess,tarfile,tempfile,threading,time,urllib.parse
p=argparse.ArgumentParser();p.add_argument('archive');a=p.parse_args()
with tempfile.TemporaryDirectory(prefix='pintle-acquired-') as td:
 root=pathlib.Path(td)
 with tarfile.open(a.archive) as tar:
  for m in tar:
   if not m.isfile() or pathlib.PurePosixPath(m.name).is_absolute() or '..' in pathlib.PurePosixPath(m.name).parts or not m.name.startswith('pintle-linux-x64/'):raise ValueError('Unsafe acquisition member')
  tar.extractall(root,filter='data')
 app=root/'pintle-linux-x64';manifest=json.loads((app/'MANIFEST.json').read_text());actual={str(f.relative_to(app)) for f in app.rglob('*') if f.is_file()}
 if actual!=set(manifest)|{'MANIFEST.json'}:raise ValueError('Unexpected payloads')
 for n,x in manifest.items():
  f=app/n
  if hashlib.sha256(f.read_bytes()).hexdigest()!=x['sha256'] or f.stat().st_size!=x['bytes'] or oct(f.stat().st_mode&0o777)!=x['mode']:raise ValueError('Payload identity mismatch')
 binary=app/'pintle';subprocess.run([str(binary),'--help'],check=True,capture_output=True)
 class Upstream(http.server.BaseHTTPRequestHandler):
  def do_GET(self):
   body=json.dumps({'path':self.path,'host':self.headers['Host']}).encode();self.send_response(200);self.end_headers();self.wfile.write(body)
  def log_message(self,*args):pass
 upstream=http.server.HTTPServer(('127.0.0.1',0),Upstream);threading.Thread(target=upstream.serve_forever,daemon=True).start()
 def port():
  with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
 https=port();plain=port();certs=root/'certs';certs.mkdir()
 subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(certs/'lvh.me-key.pem'),'-out',str(certs/'lvh.me.pem'),'-days','1','-subj','/CN=*.lvh.me','-addext','subjectAltName=DNS:*.lvh.me'],check=True,capture_output=True)
 routes=root/'routes.yaml';routes.write_text(f'routes:\n  - host: fixture.lvh.me\n    target: http://127.0.0.1:{upstream.server_port}\n    path: /service\n    strip: true\nexpect:\n  - host: fixture.lvh.me\n    why: Disposable acquired consumer\n')
 proc=subprocess.Popen([str(binary),'--static-only','--listen-port',str(https),'--http-port',str(plain),'--certs-dir',str(certs),'--routes-file',str(routes)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 context=ssl.create_default_context(cafile=str(certs/'lvh.me.pem'))
 def request(host,path):
  sock=context.wrap_socket(socket.create_connection(('127.0.0.1',https),timeout=5),server_hostname=host);sock.sendall(f'GET {path} HTTP/1.1\r\nHost: {host}:{https}\r\nConnection: close\r\n\r\n'.encode());r=http.client.HTTPResponse(sock);r.begin();body=r.read();status=r.status;sock.close();return status,body
 try:
  for _ in range(50):
   try:
    status,body=request('pintle.lvh.me','/api/health')
    if status==200:break
   except OSError:time.sleep(.1)
  else:raise ValueError('Acquired runtime did not become ready')
  status,body=request('fixture.lvh.me','/service/example?q=actual');x=json.loads(body);assert status==200 and x=={'path':'/example?q=actual','host':f'fixture.lvh.me:{https}'}
  status,body=request('pintle.lvh.me','/');assert status==200 and b'<script' in body
  status,body=request('pintle.lvh.me','/api/self');x=json.loads(body);assert status==200 and x['staticOnly'] is True and x['loopbackAddress']=='127.0.0.1' and x['expected'][0]['routed'] is True
  status,body=request('pintle.lvh.me','/api/topology');assert status==200
  for document in ['', 'routes:\n', 'routes: null\n', '{}\n']:
   routes.write_text(document);time.sleep(.6)
   status,body=request('fixture.lvh.me','/service/transient-empty');assert status==200 and json.loads(body)['path']=='/transient-empty'
  routes.write_text('routes: []\ntcp:\n  - host: db.lvh.me\n    target: 5432\n    listen: 5432\n')
  time.sleep(.5);status,body=request('fixture.lvh.me','/service/still-valid');assert status==200 and json.loads(body)['path']=='/still-valid'
  routes.write_text('routes: []\n');time.sleep(.6)
  status,body=request('fixture.lvh.me','/service/explicit-clear');assert status==404
  print(json.dumps({'payloads':len(manifest),'actualHelp':True,'actualHttpsProxyHostQueryStrip':True,'embeddedDashboard':True,'runtimeSelfAndTopology':True,'transientBlankNullAbsentAndInvalidReloadPreserveRoutes':True,'explicitEmptyClearSupported':True,'tlsVerification':'only own fixture CA; no insecure bypass or system trust mutation'}))
 finally:
  proc.terminate()
  try:proc.wait(timeout=5)
  except subprocess.TimeoutExpired:proc.kill();proc.wait()
  upstream.shutdown();upstream.server_close()
