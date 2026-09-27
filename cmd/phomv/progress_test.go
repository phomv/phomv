package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/phomv/phomv/internal/worker"
)

func TestProgressKeepsCounterOffLogLines(t *testing.T) {
	var out bytes.Buffer
	p := &progress{out: &out}

	p.Write([]byte("before any counter\n"))
	p.update(1, 10)
	p.update(2, 12)
	p.Write([]byte("WRN something\n"))
	p.finish()
	p.Write([]byte("done\n"))

	const clear = "\r\033[K"
	want := "before any counter\n" +
		"1 done / 10 found" +
		clear + "2 done / 12 found" +
		clear + "WRN something\n" + "2 done / 12 found" +
		clear +
		"done\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
}

func TestCountsAsDone(t *testing.T) {
	cases := []struct {
		r    worker.Result
		want bool
	}{
		{worker.Result{Status: worker.StatusOK}, true},
		{worker.Result{Status: worker.StatusSkippedDuplicate}, true},
		{worker.Result{Status: worker.StatusUnknownDate}, true},
		{worker.Result{Status: worker.StatusFailed, Err: errors.New("x")}, true},
		{worker.Result{Status: worker.StatusOK, SidecarOf: "IMG_1.HEIC"}, false},
		{worker.Result{Status: worker.StatusWalkError}, false},
		{worker.Result{Status: worker.StatusExcludedDir}, false},
	}
	for _, tc := range cases {
		if got := countsAsDone(tc.r); got != tc.want {
			t.Errorf("countsAsDone(%+v) = %v, want %v", tc.r, got, tc.want)
		}
	}
}
