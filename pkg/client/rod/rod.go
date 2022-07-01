package rod

import (
	"moon/pkg/config"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type Rod struct {
	*rod.Browser
	mu *sync.Mutex
}

var SETTINGS_rod_dir string = "rod"

func New() *Rod {
	l := launcher.New().
		UserDataDir(filepath.Join(SETTINGS_rod_dir, "user-data"))
	if config.DEBUG {
		l = l.Headless(false)
	}
	url := l.MustLaunch()
	rod := rod.New().ControlURL(url)
	if config.DEBUG {
		rod = rod.Trace(true).SlowMotion(2000 * time.Microsecond)
	}
	return &Rod{
		rod.MustConnect(),
		new(sync.Mutex),
	}
}

func (r *Rod) MustWaitNewPage(action func()) *rod.Page {
	r.mu.Lock()
	defer r.mu.Unlock()
	event := proto.PageWindowOpen{}
	wait := r.WaitEvent(&event)
	action()
	wait()
	time.Sleep(time.Second)
	return r.MustPages().MustFindByURL(event.URL)
}

func (r *Rod) HookDownload(action func()) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	dir := filepath.Join(SETTINGS_rod_dir, "downloads")

	wait := r.Timeout(10 * time.Second).WaitDownload(dir)
	action()
	info := wait()
	if info != nil {
		if info.SuggestedFilename == "download" {
			print("oh fuck")
			return ""
		}
		os.Rename(filepath.Join(dir, info.GUID), filepath.Join(dir, info.SuggestedFilename))
		return filepath.Join(dir, info.GUID)
	}
	return ""
}
