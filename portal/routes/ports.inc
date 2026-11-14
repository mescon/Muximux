# M-312: standalone /ports/ page + API, served by the same service as
# the M-249 lease API (127.0.0.1:3726). No dynamic Caddyfile edits for page
# routes; per-port p<port> host routes are managed via the admin API only
# and exist solely for leased ports.
handle /ports* {
	reverse_proxy 127.0.0.1:3726
}
