// +build windows

package blockexplorer

import (
	"github.com/rjeczalik/notify"
)

func init() {
	WatcherNotifies = []notify.Event{notify.All, notify.FileNotifyChangeLastWrite}
}
