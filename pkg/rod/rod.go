package rod

import (
	"moon/pkg/config"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type Rod struct {
	*rod.Browser
	mu *sync.Mutex
}

var SETTINGS_rod_dir string = "rod"

func New() *Rod {
	l := launcher.New().
		UserDataDir(filepath.Join(SETTINGS_rod_dir, "user-data"))
	if config.DEBUG && config.DEBUG_LOCAL {
		l = l.Headless(false)
	}
	url := l.MustLaunch()
	rod := rod.New().ControlURL(url)
	if config.DEBUG {
		rod = rod.Trace(true)
		if config.DEBUG_LOCAL {
			rod = rod.SlowMotion(2000 * time.Microsecond)
		}
	}
	return &Rod{
		rod.MustConnect(),
		new(sync.Mutex),
	}
}

func (r *Rod) HookDownload(action func()) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	dir := filepath.Join(SETTINGS_rod_dir, "downloads")

	wait := r.WaitDownload(dir)
	action()
	info := wait()
	if info != nil {
		if info.SuggestedFilename != "download" {
			os.Rename(filepath.Join(dir, info.GUID), filepath.Join(dir, info.SuggestedFilename))
			return filepath.Join(dir, info.SuggestedFilename)
		}
		return filepath.Join(dir, info.GUID)
	}
	return ""
}
