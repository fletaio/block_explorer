package blockexplorer

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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
	assets          *fileAsset
	sync.Mutex
}

func NewWebServer(echo *echo.Echo, assets *fileAsset, path string) *WebServer {
	web := &WebServer{
		echo:      echo,
		path:      path,
		templates: map[string]*template.Template{},
		assets:    assets,
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

func (web *WebServer) assetToData(path string) []byte {
	f, err := web.assets.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	bs, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	return bs
}

func (web *WebServer) UpdateRender() error {
	web.templates = map[string]*template.Template{}

	layout, err := web.assets.Open("layout")
	if err != nil {
		log.Fatal(err)
	}
	li, err := layout.Stat()
	if err != nil {
		log.Fatal(err)
	}
	if !li.IsDir() {
		log.Fatal("layout is not folder")
	}

	templateMap := map[string][][]byte{}
	tds := web.loadTemplates("", layout, templateMap)
	templateMap[""] = tds

	web.updateRender("", "/pages", templateMap)

	return nil
}

func (web *WebServer) loadTemplates(prefix string, layout http.File, templateMap map[string][][]byte) [][]byte {
	layoutData := web.assetToData("/layout/" + prefix + "layout.html")
	baseData := web.assetToData("/layout/" + prefix + "base.html")

	tds := [][]byte{layoutData, baseData}
	f, err := layout.Readdir(1)
	for err == nil {
		if f[0].IsDir() {
			pf := prefix + f[0].Name() + "/"
			l, err := web.assets.Open("layout/" + pf)
			if err == nil {
				tds := web.loadTemplates(pf, l, templateMap)
				templateMap[pf] = tds
			} else {
				log.Println(err)
			}
			f, err = layout.Readdir(1)
			continue
		}
		if f[0].Name() == "layout.html" || f[0].Name() == "base.html" {
			f, err = layout.Readdir(1)
			continue
		}
		tds = append(tds, web.assetToData("layout/"+prefix+f[0].Name()))
		f, err = layout.Readdir(1)
	}

	return tds

}

func (web *WebServer) updateRender(prefix, path string, templateMap map[string][][]byte) error {
	d, err := web.assets.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	var fi []os.FileInfo
	fi, err = d.Readdir(1)
	for err == nil {
		log.Println(prefix + fi[0].Name())
		if fi[0].IsDir() {
			web.updateRender(prefix+fi[0].Name()+"/", "/pages/"+fi[0].Name(), templateMap)
		} else {
			data := web.assetToData(path + "/" + fi[0].Name())

			str := string(data)
			log.Println(str)

			t := template.New(fi[0].Name())
			template.Must(t.Parse(string(data)))
			var tds [][]byte
			var has bool
			if tds, has = templateMap[prefix]; !has {
				tds = templateMap[""]
			}
			for _, td := range tds {
				template.Must(t.Parse(string(td)))
			}
			web.templates[prefix+fi[0].Name()] = t
		}

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
