# M-312: Muximux goes through portal-front (:3736), a transparent proxy that
# injects the palette extension script. Rollback: reverse_proxy 127.0.0.1:45001.
# /portal-ext/* is served by portal-front itself from lease-api (same origin).
handle {
	reverse_proxy 127.0.0.1:3736 {
		flush_interval -1
	}
}
