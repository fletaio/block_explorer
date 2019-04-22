// +build dev

package blockexplorer

import (
	"net/http"
	"runtime"
	"strings"
)

// Assets contains project assets.
var Assets http.FileSystem

func init() {
	var pwd string
	{
		pc := make([]uintptr, 10)
		runtime.Callers(1, pc)
		f := runtime.FuncForPC(pc[0])
		pwd, _ = f.FileLine(pc[0])

		path := strings.Split(pwd, "/")
		pwd = strings.Join(path[:len(path)-1], "/")
	}

	Assets = http.Dir(pwd + "/webfiles")
}
