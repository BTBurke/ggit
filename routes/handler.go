package routes

import (
	"net/http"

	"git.icyphox.sh/legit/config"
	"github.com/alexedwards/flow"
)

// Checks for gitprotocol-http(5) specific smells; if found, passes
// the request on to the git http service, else render the web frontend.
func (d *deps) Multiplex(sw string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.URL.RawQuery == "service=git-receive-pack" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("no pushing allowed!"))
			return
		}

		if sw == "info/refs" &&
			r.URL.RawQuery == "service=git-upload-pack" &&
			r.Method == "GET" {
			d.InfoRefs(w, r)
		} else if sw == "git-upload-pack" && r.Method == "POST" {
			d.UploadPack(w, r)
		} else if r.Method == "GET" {
			d.RepoIndex(w, r)
		}
	}
}

func Handlers(c *config.Config) *flow.Mux {
	mux := flow.New()
	d := deps{c}

	mux.NotFound = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		d.Write404(w)
	})

	// non-namespaced repos
	mux.HandleFunc("/", d.Index, "GET")
	mux.HandleFunc("/static/:file", d.ServeStatic, "GET")
	mux.HandleFunc("/:name", d.Multiplex(""), "GET")
	mux.HandleFunc("/:name/refs", d.Refs, "GET")
	mux.HandleFunc("/:name/tree/:ref/...", d.RepoTree, "GET")
	mux.HandleFunc("/:name/blob/:ref/...", d.FileContent, "GET")
	mux.HandleFunc("/:name/log/:ref", d.Log, "GET")
	mux.HandleFunc("/:name/commit/:ref", d.Diff, "GET")
	// git handlers
	mux.HandleFunc("/:name/info/refs", d.Multiplex("info/refs"), "GET", "POST")
	mux.HandleFunc("/:name/git-upload-pack", d.Multiplex("git-upload-pack"), "POST")

	// namespaced repos
	mux.HandleFunc("/:ns/:name", d.Multiplex(""), "GET")
	mux.HandleFunc("/:ns/:name/tree/:ref/...", d.RepoTree, "GET")
	mux.HandleFunc("/:ns/:name/blob/:ref/...", d.FileContent, "GET")
	mux.HandleFunc("/:ns/:name/log/:ref", d.Log, "GET")
	mux.HandleFunc("/:ns/:name/commit/:ref", d.Diff, "GET")
	mux.HandleFunc("/:ns/:name/refs", d.Refs, "GET")
	// git handlers
	mux.HandleFunc("/:ns/:name/info/refs", d.Multiplex("info/refs"), "GET")
	mux.HandleFunc("/:ns/:name/git-upload-pack", d.Multiplex("git-upload-pack"), "POST")

	return mux
}
