# M-249: lease API + UI (search box for any port, session-bound).
# Static route; leases themselves are Muximux apps, no dynamic Caddy routes.
handle /lease* {
	reverse_proxy 127.0.0.1:3726
}
