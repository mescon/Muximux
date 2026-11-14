# M-323: port gateway (bin/portal-gw on :3738, unit portal-gw.service).
# /p/<port>/ entry + root service worker. Rollback: `git checkout
# routes/pgw.inc` + `caddy reload` (or point back at the spike on :3738).
handle /p/* {
	@top_document {
		method GET
		header Sec-Fetch-Dest document
	}
	handle @top_document {
		reverse_proxy 127.0.0.1:3736
	}
	handle {
		reverse_proxy 127.0.0.1:3738 {
			flush_interval -1
		}
	}
}
handle /pgw-sw.js {
	reverse_proxy 127.0.0.1:3738
}
handle /sw.js {
	reverse_proxy 127.0.0.1:3738
}
handle /pgw-ping {
	reverse_proxy 127.0.0.1:3738
}
