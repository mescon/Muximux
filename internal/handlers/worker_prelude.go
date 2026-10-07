package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
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

// hasUseStrictDirective reports whether src opens with a directive
// prologue containing "use strict". Prepending the prelude pushes the
// original prologue away from the start of the script, so a strict worker
// would silently run in sloppy mode unless the directive is restated first.
//
// The prologue is the run of string-literal statements at the start of the
// script, each ended by ";", "}", the end of input, or a line break that
// does not continue the expression. Only an unescaped "use strict" counts,
// as in the language. jsUseStrictScanner is the same algorithm for the page
// interceptor; both are checked against one table of cases.
func hasUseStrictDirective(src []byte) bool {
	s := strings.TrimPrefix(string(src), "\ufeff")
	i := 0
	for {
		i, _ = skipSpaceAndComments(s, i)
		if i >= len(s) || (s[i] != '\'' && s[i] != '"') {
			return false
		}
		quote := s[i]
		j := i + 1
		for j < len(s) && s[j] != quote {
			switch s[j] {
			case '\\':
				j++
			case '\n', '\r':
				return false
			}
			j++
		}
		if j >= len(s) {
			return false
		}
		literal := s[i+1 : j]
		var newline bool
		i, newline = skipSpaceAndComments(s, j+1)
		ended := i >= len(s) || s[i] == ';' || s[i] == '}' ||
			(newline && !strings.ContainsRune(".([+-*/%,?=<>&|^`", rune(s[i])))
		if !ended {
			return false
		}
		if literal == "use strict" {
			return true
		}
		if i < len(s) && s[i] == ';' {
			i++
		}
	}
}

// skipSpaceAndComments advances past whitespace and comments from i and
// reports whether a line break was crossed. An unterminated block comment
// consumes the rest of the input.
func skipSpaceAndComments(s string, i int) (int, bool) {
	newline := false
	for i < len(s) {
		switch {
		case s[i] == '\n' || s[i] == '\r':
			newline = true
			i++
		case s[i] == ' ' || s[i] == '\t' || s[i] == '\v' || s[i] == '\f':
			i++
		case strings.HasPrefix(s[i:], "//"):
			end := strings.IndexByte(s[i:], '\n')
			if end < 0 {
				return len(s), newline
			}
			i += end
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return len(s), newline
			}
			if strings.ContainsAny(s[i:i+2+end], "\n\r") {
				newline = true
			}
			i += 2 + end + 2
		default:
			return i, newline
		}
	}
	return i, newline
}

// jsUseStrictScanner is hasUseStrictDirective in JavaScript, embedded in the
// page interceptor to rebuild blob workers. Kept to the same algorithm and
// tested against the same cases; \x60 is a backtick, which the Go raw
// string cannot contain.
const jsUseStrictScanner = `function(s){if(s.charCodeAt(0)===0xfeff)s=s.slice(1);var i=0,n=s.length,nl;` +
	`function sk(){nl=false;while(i<n){var c=s[i];` +
	`if(c==="\n"||c==="\r"){nl=true;i++}` +
	`else if(c===" "||c==="\t"||c==="\v"||c==="\f")i++;` +
	`else if(c==="/"&&s[i+1]==="/"){var e=s.indexOf("\n",i);if(e<0){i=n;return}i=e}` +
	`else if(c==="/"&&s[i+1]==="*"){var f=s.indexOf("*/",i+2);if(f<0){i=n;return}if(/[\n\r]/.test(s.slice(i,f)))nl=true;i=f+2}` +
	`else return}}` +
	`for(;;){sk();if(i>=n)return false;var q=s[i];if(q!=="'"&&q!=='"')return false;` +
	`var j=i+1;for(;j<n&&s[j]!==q;j++){if(s[j]==="\\")j++;else if(s[j]==="\n"||s[j]==="\r")return false}` +
	`if(j>=n)return false;var l=s.slice(i+1,j);i=j+1;sk();var c=s[i];` +
	`if(!(i>=n||c===";"||c==="}"||(nl&&".([+-*/%,?=<>&|^\x60".indexOf(c)<0)))return false;` +
	`if(l==="use strict")return true;if(c===";")i++}}`

// prependWorkerPrelude returns src with the worker prelude in front,
// restating "use strict" first when src opens with that directive.
func (r *contentRewriter) prependWorkerPrelude(src []byte) []byte {
	prelude := r.workerPrelude()
	var out bytes.Buffer
	out.Grow(len(prelude) + len(src) + 16)
	if hasUseStrictDirective(src) {
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
