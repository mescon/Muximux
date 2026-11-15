# M-312: Muximux goes through portal-front (:3736), a transparent proxy that
# injects the palette extension script. Rollback: reverse_proxy 127.0.0.1:45001.
# /portal-ext/* is served by portal-front itself from lease-api (same origin).
handle {
	# M-341 (Lead review): same encode as routes/pgw.inc. The Referer-fallback
	# path (root-relative app assets via portal-front -> gateway, before the SW
	# controls the page) bypasses /p/*, so without this it serves identity
	# (measured 939,867 B vs 294,632 B gzip for dsh-x index bundle). Typed
	# match, one value per `header` line, `*` suffix; no text/event-stream so
	# SSE keeps streaming under flush_interval -1; WS 101s never encoded.
	# Rollback: `git checkout routes/front.inc` + `systemctl --user reload portal-caddy`.
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
	reverse_proxy 127.0.0.1:3736 {
		flush_interval -1
	}
}
