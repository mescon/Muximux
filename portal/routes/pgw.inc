# M-323: port gateway (bin/portal-gw on :3738, unit portal-gw.service).
# /p/<port>/ entry + root service worker. Rollback: `git checkout
# routes/pgw.inc` + `caddy reload` (or point back at the spike on :3738).
handle /p/* {
	# M-341: Caddy compresses gateway responses (upstreams serve identity:
	# portal-gw strips Accept-Encoding). One value per `header` line (the
	# header matcher takes a single value; space-separated lists do not
	# parse); `*` suffix needed — upstream Content-Types carry `; charset=`
	# params and bare values do not match (verified live). Multiple lines
	# for the same header are OR'd (verified live). No text/event-stream
	# line, so SSE keeps streaming under flush_interval -1. WS upgrades
	# (101 Switching Protocols) are never encoded.
	encode zstd gzip {
		match {
			header Content-Type text/html*
			header Content-Type text/css*
			header Content-Type text/javascript*
			header Content-Type application/javascript*
			header Content-Type application/json*
			header Content-Type application/xml*
			header Content-Type text/plain*
			header Content-Type text/xml*
			header Content-Type image/svg+xml*
			header Content-Type application/wasm*
			header Content-Type application/manifest+json*
			header Content-Type font/woff*
			header Content-Type font/woff2*
		}
	}
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
