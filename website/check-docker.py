#!/usr/bin/env python3
"""Real dedicated-runner Docker label discovery using the acquired customer binary.
No production daemon: GitHub Actions Ubuntu runner only; exact fixture IDs cleaned.
"""
import argparse, http.client, json, os, pathlib, socket, ssl, subprocess, tempfile, time

p = argparse.ArgumentParser()
p.add_argument('binary')
a = p.parse_args()
if os.environ.get('GITHUB_ACTIONS') != 'true' or os.environ.get('RUNNER_OS') != 'Linux':
    raise SystemExit('Requires a dedicated Linux GitHub Actions runner')
binary = str(pathlib.Path(a.binary).resolve(strict=True))

def run(*args):
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout.strip()

def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]

run('docker', 'info', '--format', '{{.ServerVersion}}')
with tempfile.TemporaryDirectory(prefix='pintle-docker-') as td:
    root = pathlib.Path(td)
    suffix = root.name.removeprefix('pintle-docker-')
    network, image = 'pintle-fixture-' + suffix, 'pintle-fixture:' + suffix
    owned, net_created, image_created, proc = [], False, False, None
    try:
        (root/'upstream.go').write_text('''package main
import("encoding/json";"net/http";"os")
func main(){http.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]string{"marker":os.Getenv("MARKER"),"path":r.URL.RequestURI(),"host":r.Host})});if err:=http.ListenAndServe(":8080",nil);err!=nil{panic(err)}}
''')
        subprocess.run(['go','build','-o',str(root/'upstream'),str(root/'upstream.go')],env={**os.environ,'CGO_ENABLED':'0'},check=True,capture_output=True)
        (root/'Dockerfile').write_text('FROM scratch\nCOPY upstream /upstream\nEXPOSE 8080\nENTRYPOINT ["/upstream"]\n')
        run('docker', 'build', '--label', 'pintle.fixture='+suffix, '-t', image, str(root))
        image_created = True
        run('docker', 'network', 'create', '--label', 'pintle.fixture='+suffix, network)
        net_created = True
        labels = {
            'native': {'pintle.host':'native.lvh.me', 'pintle.port':'8080', 'pintle.path':'/service', 'pintle.strip':'true'},
            'traefik': {'traefik.enable':'true', 'traefik.http.routers.fixture.rule':'Host(`traefik.lvh.me`)', 'traefik.http.services.fixture.loadbalancer.server.port':'8080'},
            'caddy': {'caddy':'caddy.lvh.me', 'caddy.reverse_proxy':'{{upstreams 8080}}'},
        }
        for marker, fields in labels.items():
            args=['docker','run','-d','--network',network,'--name',network+'-'+marker,'--label','pintle.fixture='+suffix,'-e','MARKER='+marker]
            for key,value in fields.items(): args += ['--label',key+'='+value]
            owned.append(run(*args,image))
        certs=root/'certs';certs.mkdir()
        run('openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(certs/'lvh.me-key.pem'),'-out',str(certs/'lvh.me.pem'),'-days','1','-subj','/CN=*.lvh.me','-addext','subjectAltName=DNS:*.lvh.me')
        routes=root/'routes.yaml';routes.write_text('routes: []\n')
        https,plain=port(),port()
        env={**os.environ,'DOCKER_NETWORK':network,'BASE_DOMAIN':'lvh.me'}
        log=(root/'proxy.log').open('w')
        proc=subprocess.Popen([binary,'--listen-port',str(https),'--http-port',str(plain),'--certs-dir',str(certs),'--routes-file',str(routes)],env=env,stdout=log,stderr=log)
        context=ssl.create_default_context(cafile=str(certs/'lvh.me.pem'))
        def request(host,path):
            with context.wrap_socket(socket.create_connection(('127.0.0.1',https),timeout=3),server_hostname=host) as sock:
                sock.sendall(f'GET {path} HTTP/1.1\r\nHost: {host}:{https}\r\nConnection: close\r\n\r\n'.encode())
                response=http.client.HTTPResponse(sock);response.begin()
                return response.status,response.read()
        def wait(check):
            for _ in range(100):
                if proc.poll() is not None: raise RuntimeError('Pintle exited: '+(root/'proxy.log').read_text())
                try:
                    if check(): return
                except (OSError,ValueError,http.client.HTTPException): pass
                time.sleep(.2)
            raise AssertionError('Actual Docker route state did not converge')
        for marker in labels:
            host=marker+'.lvh.me';path='/service/actual?q=docker' if marker=='native' else '/actual?q=docker'
            expected={'marker':marker,'path':'/actual?q=docker','host':host+':'+str(https)}
            def matches():
                status,body=request(host,path)
                return status==200 and json.loads(body)==expected
            wait(matches)
        status,body=request('pintle.lvh.me','/api/topology')
        assert status==200 and all(x.encode() in body for x in ['native.lvh.me','traefik.lvh.me','caddy.lvh.me'])
        removed=owned[0];run('docker','rm','-f',removed);owned.remove(removed)
        wait(lambda:request('native.lvh.me','/service/removed')[0]==404)
        assert request('traefik.lvh.me','/survives')[0]==200
        print(json.dumps({'realDockerDiscovery':['pintle','traefik','caddy'],'hostQueryAndNativeStrip':True,'actualTopology':True,'containerRemovalReload':True,'remainingContainerRetained':True,'tls':'own client CA only','dedicatedRunnerOnly':True}))
    finally:
        if proc:
            proc.terminate()
            try:proc.wait(timeout=5)
            except subprocess.TimeoutExpired:proc.kill();proc.wait()
        for cid in owned:
            subprocess.run(['docker','rm','-f',cid],check=False,capture_output=True)
        if net_created:subprocess.run(['docker','network','rm',network],check=False,capture_output=True)
        if image_created:subprocess.run(['docker','image','rm',image],check=False,capture_output=True)
