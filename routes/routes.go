package routes

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"git.icyphox.sh/legit/config"
	"git.icyphox.sh/legit/git"
	"github.com/alexedwards/flow"
	"github.com/dustin/go-humanize"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type deps struct {
	c *config.Config
}

type info struct {
	Name, Idle string
	Desc       template.HTML
	d          time.Time
}

func scanPath(basedir string, dir string) []info {
	d := filepath.Join(basedir, dir)
	dirs, err := os.ReadDir(d)
	if err != nil {
		return []info{}
	}
	infos := []info{}

	for _, d1 := range dirs {
		if !d1.IsDir() {
			continue
		}
		path := filepath.Join(d, d1.Name())
		gr, err := git.Open(path, "")
		if err != nil {
			// non-git directory, scan up to one directory deeper
			if dir == "" {
				subInfo := scanPath(basedir, d1.Name())
				infos = append(infos, subInfo...)
			}
			continue
		}
		c, err := gr.LastCommit()
		if err != nil {
			continue
		}
		desc := getDescription(path)

		infos = append(infos, info{
			Name: filepath.Join(dir, d1.Name()),
			Desc: desc,
			Idle: humanize.Time(c.Author.When),
			d:    c.Author.When,
		})
	}
	return infos
}

func (d *deps) Index(w http.ResponseWriter, r *http.Request) {
	infos := scanPath(d.c.Repo.ScanPath, "")

	sort.Slice(infos, func(i, j int) bool {
		return infos[j].d.Before(infos[i].d)
	})

	indexPage(d.c, infos).Render(r.Context(), w)
}

func (d *deps) RepoIndex(w http.ResponseWriter, r *http.Request) {
	name := flow.Param(r.Context(), "name")
	ns := flow.Param(r.Context(), "ns")

	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	name = filepath.Clean(filepath.Join(ns, name))
	path := filepath.Join(d.c.Repo.ScanPath, name)

	gr, err := git.Open(path, "")
	if err != nil {
		d.Write404(w)
		return
	}

	commits, err := gr.Commits()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	var readmeContent template.HTML
	for _, readme := range d.c.Repo.Readme {
		ext := filepath.Ext(readme)
		content, _ := gr.FileContent(readme)
		if len(content) > 0 {
			switch ext {
			case ".md", ".mkd", ".markdown":
				unsafe := blackfriday.Run(
					[]byte(content),
					blackfriday.WithExtensions(blackfriday.CommonExtensions),
				)
				html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
				readmeContent = template.HTML(html)
			default:
				readmeContent = template.HTML(
					fmt.Sprintf(`<pre>%s</pre>`, content),
				)
			}
			break
		}
	}

	if readmeContent == "" {
		log.Printf("no readme found for %s", name)
	}

	mainBranch, err := gr.FindMainBranch(d.c.Repo.MainBranch)
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]any)
	data["lenCommits"] = len(commits) - 1

	// contrain display to up to 3 commits
	if len(commits) >= 3 {
		commits = commits[:3]
	}

	data["name"] = name
	data["ref"] = mainBranch
	data["readme"] = readmeContent
	data["commits"] = commits
	data["desc"] = getDescription(path)
	data["servername"] = d.c.Server.Name
	data["meta"] = d.c.Meta
	data["gomod"] = isGoModule(gr)

	if err := t.ExecuteTemplate(w, "repo", data); err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	return
}

func (d *deps) RepoTree(w http.ResponseWriter, r *http.Request) {
	ns := flow.Param(r.Context(), "ns")
	name := flow.Param(r.Context(), "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	treePath := flow.Param(r.Context(), "...")
	ref := flow.Param(r.Context(), "ref")

	name = filepath.Clean(filepath.Join(ns, name))
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	files, err := gr.FileTree(treePath)
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	data := make(map[string]any)
	data["name"] = name
	data["ref"] = ref
	data["parent"] = treePath
	data["desc"] = getDescription(path)
	data["dotdot"] = filepath.Dir(treePath)

	d.listFiles(files, data, w)
	return
}

func (d *deps) FileContent(w http.ResponseWriter, r *http.Request) {
	ns := flow.Param(r.Context(), "ns")
	name := flow.Param(r.Context(), "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	treePath := flow.Param(r.Context(), "...")
	ref := flow.Param(r.Context(), "ref")

	name = filepath.Clean(filepath.Join(ns, name))
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	contents, err := gr.FileContent(treePath)
	data := make(map[string]any)
	data["name"] = name
	data["ref"] = ref
	data["desc"] = getDescription(path)
	data["path"] = treePath

	d.showFile(contents, data, w)
	return
}

func (d *deps) Log(w http.ResponseWriter, r *http.Request) {
	ns := flow.Param(r.Context(), "ns")
	name := flow.Param(r.Context(), "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := flow.Param(r.Context(), "ref")

	name = filepath.Clean(filepath.Join(ns, name))
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	commits, err := gr.Commits()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})
	data["commits"] = commits
	data["meta"] = d.c.Meta
	data["name"] = name
	data["ref"] = ref
	data["desc"] = getDescription(path)
	data["log"] = true

	if err := t.ExecuteTemplate(w, "log", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) Diff(w http.ResponseWriter, r *http.Request) {
	ns := flow.Param(r.Context(), "ns")
	name := flow.Param(r.Context(), "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := flow.Param(r.Context(), "ref")

	name = filepath.Clean(filepath.Join(ns, name))
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	diff, err := gr.Diff()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})

	data["commit"] = diff.Commit
	data["stat"] = diff.Stat
	data["diff"] = diff.Diff
	data["meta"] = d.c.Meta
	data["name"] = name
	data["ref"] = ref
	data["desc"] = getDescription(path)

	if err := t.ExecuteTemplate(w, "commit", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) Refs(w http.ResponseWriter, r *http.Request) {
	ns := flow.Param(r.Context(), "ns")
	name := flow.Param(r.Context(), "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}

	name = filepath.Clean(filepath.Join(ns, name))
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, "")
	if err != nil {
		d.Write404(w)
		return
	}

	tags, err := gr.Tags()
	if err != nil {
		// Non-fatal, we *should* have at least one branch to show.
		log.Println(err)
	}

	branches, err := gr.Branches()
	if err != nil {
		log.Println(err)
		d.Write500(w)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})

	data["meta"] = d.c.Meta
	data["name"] = name
	data["branches"] = branches
	data["tags"] = tags
	data["desc"] = getDescription(path)

	if err := t.ExecuteTemplate(w, "refs", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) ServeStatic(w http.ResponseWriter, r *http.Request) {
	f := flow.Param(r.Context(), "file")
	f = filepath.Clean(filepath.Join(d.c.Dirs.Static, f))

	http.ServeFile(w, r, f)
}
