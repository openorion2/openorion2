package host

import (
	"fmt"
	"image/png"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// TestControl: the screen comes as a PNG of the frame shown; a click glides
// the pointer there and buffers a press on the spot; keys arrive.
func TestControl(t *testing.T) {
	t.Parallel()
	addr := freeAddr(t)
	got := make(chan string, 4)
	go Run(Options{Headless: true, Control: addr, Frames: 60 * 60}, func(sys *System) error {
		page := NewScreen()
		page.Palette[1].R = 0xff
		page.Pix[0] = 1
		for {
			if err := sys.Flip(page); err != nil {
				return err
			}
			if sys.MouseBuffer() {
				p := sys.BufferedPress()
				got <- fmt.Sprintf("press %d,%d %d", p.X, p.Y, p.Buttons)
			}
			if k, ok := sys.PopKey(); ok {
				got <- fmt.Sprintf("key %d", k.Scancode)
			}
		}
	})
	url := "http://" + addr
	var resp *http.Response
	var err error
	for range 100 {
		if resp, err = http.Get(url + "/screen.png"); err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if err == nil {
			resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := img.At(0, 0).RGBA(); r>>8 != 0xff {
		t.Errorf("pixel 0,0 red %#x", r>>8)
	}
	post := func(path string) {
		t.Helper()
		resp, err := http.Post(url+path, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %s", path, resp.Status)
		}
	}
	post("/click?x=300&y=200")
	if g := <-got; g != "press 300,200 1" {
		t.Errorf("click: %s", g)
	}
	post("/click?x=10&y=20&button=right")
	if g := <-got; g != "press 10,20 2" {
		t.Errorf("right click: %s", g)
	}
	post("/key?name=enter")
	if g := <-got; g != "key 40" {
		t.Errorf("key: %s", g)
	}
}

func TestParseKeys(t *testing.T) {
	t.Parallel()
	ks, err := ParseKeys("alt+ab")
	if err != nil || len(ks) != 2 || !ks[0].Alt || ks[1].Scancode != 5 || ks[1].Code != 'b' {
		t.Errorf("alt+ab: %+v, %v", ks, err)
	}
	if ks, err := ParseKeys("f10"); err != nil || len(ks) != 1 || ks[0].Scancode != 67 {
		t.Errorf("f10: %+v, %v", ks, err)
	}
	if _, err := ParseKeys("bogus!"); err == nil {
		t.Error("bogus! parsed")
	}
}
