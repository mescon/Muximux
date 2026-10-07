package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
)

// The page interceptor (interceptorScript) only runs in the proxied app's
// documents. Web Workers have their own global scope, so a worker's fetch,
// XHR, WebSocket, EventSource and importScripts calls reach the Muximux
// origin unprefixed: a root-relative "/api/x" resolves against the origin
// root, and a worker started from a blob: URL cannot resolve root-relative
// URLs at all. Dispatcharr's player hits this: it builds
// `${location.origin}/proxy/ts/stream/<uuid>` and mpegts.js fetches it from a
// blob: worker, so the request arrived as /proxy/ts/stream/<uuid> and was
// answered with "App not found: ts".
//
// workerPrelude is a small script run at the top of every worker a proxied
// app starts. It is prepended in two places:
//   - by the proxy, to script responses the browser fetches as a worker
//     (Sec-Fetch-Dest: worker / sharedworker), and
//   - by the page interceptor, which restarts blob: workers from a new blob
//     with the prelude in front (blob workers never pass through the proxy).
//
// A referer-based fallback on the server cannot cover this: browsers send no
// Referer from blob: workers, and the page's own Referer no longer carries
// the proxy prefix because the interceptor keeps the address clean.

// workerPrelude returns the worker-scope interceptor for this app.
//
// Inside the worker, the page origin comes from self.location, or from the
// URL embedded in a blob: location ("blob:https://host/uuid"). Root-relative
// URLs are made absolute because a blob worker cannot resolve them, and
// same-host absolute URLs outside the prefix get it added. Other hosts and
// relative URLs are left alone. The guard flag makes a second evaluation in
// the same scope a no-op so the prefix is never applied twice.
func (r *contentRewriter) workerPrelude() []byte {
	return []byte(`(function(){if(self.__muximuxWorker)return;self.__muximuxWorker=1;` +
		`var P="` + r.proxyPrefix + `",L=self.location,H=L.host,O=L.protocol+"//"+L.host;` +
		`if(L.protocol==="blob:"){try{var b=new URL(L.pathname);H=b.host;O=b.protocol+"//"+b.host}catch(e){}}` +
		`function R(u){if(u instanceof URL)u=u.href;if(typeof u!=="string")return u;` +
		`if(u[0]==="/"&&u[1]!=="/")return u===P||u.indexOf(P+"/")===0?O+u:O+P+u;` +
		`try{var p=new URL(u);if(p.host===H&&/^(https?|wss?):$/.test(p.protocol)&&p.pathname!==P&&p.pathname.indexOf(P+"/")!==0){p.pathname=P+p.pathname;return p.href}}catch(e){}` +
		`return u}` +
		`var F=self.fetch;if(F)self.fetch=function(i,o){` +
		`if(typeof i==="string"||i instanceof URL)i=R(i);` +
		`else if(typeof Request!=="undefined"&&i instanceof Request){var n=R(i.url);if(n!==i.url)i=new Request(n,i)}` +
		`return F.call(this,i,o)};` +
		`if(self.XMLHttpRequest){var X=self.XMLHttpRequest.prototype.open;` +
		`self.XMLHttpRequest.prototype.open=function(){var a=[].slice.call(arguments);a[1]=R(a[1]);return X.apply(this,a)}}` +
		`if(self.WebSocket){var W=self.WebSocket;self.WebSocket=function(u,q){return q!==void 0?new W(R(u),q):new W(R(u))};` +
		`self.WebSocket.prototype=W.prototype;["CONNECTING","OPEN","CLOSING","CLOSED"].forEach(function(k){self.WebSocket[k]=W[k]})}` +
		`if(self.EventSource){var E=self.EventSource;self.EventSource=function(u,c){return new E(R(u),c)};` +
		`self.EventSource.prototype=E.prototype}` +
		`if(self.importScripts){var I=self.importScripts;self.importScripts=function(){return I.apply(this,[].map.call(arguments,R))}}` +
		`})();`)
}

// workerPreludeJSLiteral is the prelude as a JavaScript string literal for
// embedding in the page interceptor. json.Marshal escapes <, > and &, so the
// literal can never close the surrounding inline <script>.
func (r *contentRewriter) workerPreludeJSLiteral() string {
	b, _ := json.Marshal(string(r.workerPrelude())) //nolint:errchkjson // marshalling a string cannot fail
	return string(b)
}

// strictDirective matches a leading "use strict" directive, after any
// comments. Prepending the prelude would otherwise push the directive out
// of the directive prologue and silently run a strict worker in sloppy mode.
var strictDirective = regexp.MustCompile(`^\s*(?:(?://[^\n]*\n|/\*[\s\S]*?\*/)\s*)*(?:'use strict'|"use strict")`)

// prependWorkerPrelude returns src with the worker prelude in front,
// restating "use strict" first when src opens with that directive.
func (r *contentRewriter) prependWorkerPrelude(src []byte) []byte {
	prelude := r.workerPrelude()
	var out bytes.Buffer
	out.Grow(len(prelude) + len(src) + 16)
	if strictDirective.Match(src) {
		out.WriteString(`"use strict";`)
	}
	out.Write(prelude)
	out.WriteByte('\n')
	out.Write(src)
	return out.Bytes()
}

// isWorkerScriptRequest reports whether the browser fetched this response as
// a dedicated or shared worker's script.
func isWorkerScriptRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	switch req.Header.Get("Sec-Fetch-Dest") {
	case "worker", "sharedworker":
		return true
	}
	return false
}
