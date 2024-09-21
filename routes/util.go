package routes

import (
	"html/template"
	"os"
	"path/filepath"

	"git.icyphox.sh/legit/git"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

func isGoModule(gr *git.GitRepo) bool {
	_, err := gr.FileContent("go.mod")
	return err == nil
}

func getDescription(path string) (desc template.HTML) {
	content, err := os.ReadFile(filepath.Join(path, "description"))
	if err == nil {
		if len(content) > 0 {
			unsafe := blackfriday.Run(
				[]byte(content),
				blackfriday.WithExtensions(blackfriday.CommonExtensions),
			)
			html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
			desc = template.HTML(html)
		}
	}
	return
}

func (d *deps) isIgnored(name string) bool {
	for _, i := range d.c.Repo.Ignore {
		if name == i {
			return true
		}
	}

	return false
}
