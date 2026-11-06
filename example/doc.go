// Package example is mise-check's worked example of a mise tool whose users are Go repos: greet, one Go module with no
// binary (its command, cmd/greet, which Go builds when a repo runs go tool greet, and its package, greet/), its template
// (templates/, embedded by templates/templates.go, as mise-cloudflare's is), and its walk on the template and on a new
// repo (walk_test.go). Read README.md here.
package example
