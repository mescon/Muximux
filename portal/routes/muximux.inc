# M-195: portal = Muximux (Cloudflare Access is the only gate). Websockets pass through.
handle {
	reverse_proxy 127.0.0.1:45001 {
		flush_interval -1
	}
}
