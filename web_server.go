package blockexplorer

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/labstack/echo"
)

type WebServer struct {
	path            string
	hasWatch        bool
	templates       map[string]*template.Template
	echo            *echo.Echo
	isRequireReload bool
	sync.Mutex
}

func NewWebServer(echo *echo.Echo, path string) *WebServer {
	web := &WebServer{
		echo:      echo,
		path:      path,
		templates: map[string]*template.Template{},
	}

	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		WebPath, err := filepath.Abs(path)
		if err != nil {
			log.Fatalln(err)
		}

		NewFileWatcher(WebPath, func(ev string, path string) {
			if strings.HasPrefix(filepath.Ext(path), ".htm") {
				web.isRequireReload = true
			}
		})
		web.hasWatch = true
	}
	web.UpdateRender()

	return web
}

func (web *WebServer) CheckWatch() {
	if web.isRequireReload {
		web.Lock()
		if web.isRequireReload {
			err := web.UpdateRender()
			if err != nil {
				log.Println(err)
			} else {
				web.isRequireReload = false
			}
		}
		web.Unlock()
	}
}

func (web *WebServer) UpdateRender() error {
	web.templates = map[string]*template.Template{}

	layout, err := Assets.Open("/layout/layout.html")
	if err != nil {
		return err
	}
	layoutData, err := ioutil.ReadAll(layout)
	if err != nil {
		return err
	}

	base, err := Assets.Open("/layout/base.html")
	if err != nil {
		return err
	}
	baseData, err := ioutil.ReadAll(base)
	if err != nil {
		return err
	}

	d, err := Assets.Open("/pages")
	if err != nil {
		log.Fatal(err)
	}
	var fi []os.FileInfo
	fi, err = d.Readdir(1)
	for err == nil {
		f, err2 := Assets.Open("/pages/" + fi[0].Name())
		if err2 != nil {
			return err2
		}
		data, err2 := ioutil.ReadAll(f)
		if err2 != nil {
			return err2
		}

		t := template.New(fi[0].Name())
		template.Must(t.Parse(string(data)))
		template.Must(t.Parse(string(layoutData)))
		template.Must(t.Parse(string(baseData)))
		web.templates[fi[0].Name()] = t

		fi, err = d.Readdir(1)
	}

	return nil
}

func (web *WebServer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	tmpl, ok := web.templates[name]
	if !ok {
		err := errors.New("Template not found -> " + name)
		return err
	}
	return tmpl.ExecuteTemplate(w, "base.html", data)
}
