package rod

import (
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
		//Headless(false).
		//Devtools(false).
		UserDataDir(filepath.Join(SETTINGS_rod_dir, "user-data"))
	url := l.MustLaunch()
	return &Rod{
		rod.New().
			ControlURL(url).
			//Trace(true).
			//SlowMotion(500 * time.Microsecond).
			MustConnect(),
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

	wait := r.Timeout(5 * time.Second).WaitDownload(dir)
	action()
	info := wait()
	if info != nil {
		os.Rename(filepath.Join(dir, info.GUID), filepath.Join(dir, info.SuggestedFilename))
		return filepath.Join(dir, info.SuggestedFilename)
	}
	return ""
}
