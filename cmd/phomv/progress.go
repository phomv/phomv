package main

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// progress keeps a live "N done / M found" counter on the last line of a
// terminal. Log output is routed through Write, which clears the counter
// before each log line and redraws it after, so the two never interleave.
type progress struct {
	mu    sync.Mutex
	out   io.Writer
	line  string // counter text; "" when none should be shown
	shown bool   // counter is currently on screen
}

func (p *progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.erase()
	n, err := p.out.Write(b)
	p.draw()
	return n, err
}

// update redraws the counter with new totals.
func (p *progress) update(done, found uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.erase()
	p.line = fmt.Sprintf("%d done / %d found", done, found)
	p.draw()
}

// finish removes the counter for good.
func (p *progress) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.erase()
	p.line = ""
}

func (p *progress) erase() {
	if p.shown {
		fmt.Fprint(p.out, "\r\033[K")
		p.shown = false
	}
}

func (p *progress) draw() {
	if p.line != "" {
		fmt.Fprint(p.out, p.line)
		p.shown = true
	}
}

// isTerminal reports whether f is an interactive terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
